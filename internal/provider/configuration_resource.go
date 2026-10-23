package provider

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	configs "github.com/F5Networks/terraform-provider-f5ads/internal/provider/clients/configs/2026-07-31"
	naas "github.com/F5Networks/terraform-provider-f5ads/internal/provider/clients/naas"
	objects "github.com/F5Networks/terraform-provider-f5ads/internal/provider/objects"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	nameMinLength       = 3
	nameMaxLength       = 30
	nameSuffixLength    = 9
	namePrefixMaxLength = nameMaxLength - nameSuffixLength

	// The API only accepts `/etc/nginx/nginx.conf` as the root configuration under the `conf_path` field
	// and specifying anything else leads to a failure.
	// The provider will always set this path without exposing it as a user input.
	nginxConfPath = "/etc/nginx/nginx.conf"
)

// nameRegex matches the names accepted by F5 ADS: they must start with a
// lowercase letter, end with a lowercase letter or digit, and otherwise
// contain only lowercase letters, digits, and hyphens.
var nameRegex = regexp.MustCompile(`^[a-z][a-z0-9-]*[a-z0-9]$`)

// namePrefixRegex matches valid prefixes.
var namePrefixRegex = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// Ensure configResource satisfies the expected interfaces.
var (
	_ resource.Resource                     = &configResource{}
	_ resource.ResourceWithConfigure        = &configResource{}
	_ resource.ResourceWithImportState      = &configResource{}
	_ resource.ResourceWithConfigValidators = &configResource{}
)

// NewConfigResource is a helper function to simplify the provider implementation.
func NewConfigResource() resource.Resource {
	return &configResource{}
}

// configResource is the resource implementation.
type configResource struct {
	client *configs.ClientWithResponses
}

// configResourceModel maps the resource schema data.
type configResourceModel struct {
	Id              types.String         `tfsdk:"id"`
	Name            types.String         `tfsdk:"name"`
	NamePrefix      types.String         `tfsdk:"name_prefix"`
	Description     types.String         `tfsdk:"description"`
	OrganizationID  types.String         `tfsdk:"organization_id"`
	LatestVersionID types.String         `tfsdk:"latest_version_id"`
	Configs         []ConfigGroupModel   `tfsdk:"configs"`
	Payloads        []ConfigPayloadModel `tfsdk:"payloads"`
}

// ConfigPayloadModel references a managed certificate or its private key.
type ConfigPayloadModel struct {
	Type     types.String   `tfsdk:"type"`
	ObjectID types.String   `tfsdk:"object_id"`
	Paths    []types.String `tfsdk:"paths"`
}

// ConfigGroupModel represents a group of F5 ADS configuration files.
type ConfigGroupModel struct {
	Name  types.String      `tfsdk:"name"`
	Files []ConfigFileModel `tfsdk:"files"`
}

// ConfigFileModel represents a single F5 ADS configuration file.
type ConfigFileModel struct {
	Name     types.String `tfsdk:"name"`
	Contents types.String `tfsdk:"contents"`
}

// Metadata returns the name of the resource.
func (r *configResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_configuration"
}

// Schema defines the resource schema.
func (r *configResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an F5 ADS configuration. A configuration is a named, versioned set of " +
			"files grouped by the directory they belong to.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Unique identifier for the configuration, assigned by F5 ADS.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization_id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier of the organization that owns the F5 ADS configuration.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Unique name for the F5 ADS configuration. Must be 3-30 characters, start with a " +
					"lowercase letter, end with a lowercase letter or digit, and contain only lowercase " +
					"letters, digits, and hyphens. Exactly one of `name` or `name_prefix` must be set. ",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthBetween(nameMinLength, nameMaxLength),
					stringvalidator.RegexMatches(
						nameRegex,
						"must start with a lowercase letter, end with a lowercase letter or digit, and contain "+
							"only lowercase letters, digits, and hyphens",
					),
				},
			},
			"name_prefix": schema.StringAttribute{
				Optional: true,
				Description: "Creates a unique name for the F5 ADS configuration beginning with this prefix. " +
					"Must start with a lowercase letter and contain only lowercase letters, digits, and hyphens. " +
					"Exactly one of `name` or `name_prefix` must be set.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, namePrefixMaxLength),
					stringvalidator.RegexMatches(
						namePrefixRegex,
						"must start with a lowercase letter and contain only lowercase letters, digits, and hyphens",
					),
				},
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Description of the F5 ADS configuration.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"latest_version_id": schema.StringAttribute{
				Computed: true,
				Description: "Identifier of the latest version of the NGINX configuration. " +
					"Updating configuration files will create a new version.",
			},
			"payloads": schema.ListNestedAttribute{
				Optional:    true,
				Description: "Managed certificates and private keys to deploy with this configuration.",
				Validators:  []validator.List{listvalidator.SizeAtLeast(1)},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"type": schema.StringAttribute{
							Required:    true,
							Description: "Payload type: `managed_certificate` or `managed_key`.",
							Validators: []validator.String{
								stringvalidator.OneOf(string(configs.NginxConfigPayloadManagedCertificate), string(configs.NginxConfigPayloadManagedKey)),
							},
						},
						"object_id": schema.StringAttribute{
							Optional:    true,
							Description: "ID of an `f5ads_certificate` resource.",
						},
						"paths": schema.ListAttribute{
							Required:    true,
							ElementType: types.StringType,
							Validators: []validator.List{
								listvalidator.SizeAtLeast(1),
								listvalidator.ValueStringsAre(stringvalidator.LengthAtLeast(1)),
							},
							Description: "File paths where the certificate or private key will be deployed. " +
								"Use these paths in the NGINX ssl_certificate or ssl_certificate_key directives.",
						},
					},
				},
			},
			"configs": schema.SetNestedAttribute{
				Required:    true,
				Description: "Groups of F5 ADS configuration files. Each group represents a directory containing one or more configuration files.",
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required:    true,
							Description: "Directory path that the files in this group belong to (e.g. \"/etc/nginx\").",
							Validators: []validator.String{
								stringvalidator.LengthAtLeast(1),
							},
						},
						"files": schema.SetNestedAttribute{
							Required:    true,
							Description: "NGINX configuration files that live under the directory.",
							Validators: []validator.Set{
								setvalidator.SizeAtLeast(1),
							},
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"name": schema.StringAttribute{
										Required:    true,
										Description: "File name relative to the containing directory (e.g. \"nginx.conf\").",
										Validators: []validator.String{
											stringvalidator.LengthAtLeast(1),
										},
									},
									"contents": schema.StringAttribute{
										Required:    true,
										Description: "Base64-encoded contents of the file.",
										Validators: []validator.String{
											stringvalidator.LengthAtLeast(1),
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// Configure adds the provider configured API clients to the resource.
func (r *configResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Add a nil check to ensure the provider has been configured.
	if req.ProviderData == nil {
		return
	}

	clients, ok := req.ProviderData.(*apiClients)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *apiClients, got: %T. Please report this issue to the provider.", req.ProviderData),
		)
		return
	}

	r.client = clients.configs
}

func (r *configResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(
			path.MatchRoot("name"),
			path.MatchRoot("name_prefix"),
		),
	}
}

// Create creates the resource and sets in the terraform configuration.
func (r *configResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan configResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// When user input the name_prefix instead of name, generate a
	// unique name by appending a random suffix to the prefix.
	if plan.Name.IsUnknown() {
		name, err := generateNameWithPrefix(plan.NamePrefix.ValueString())
		if err != nil {
			resp.Diagnostics.AddError(
				"Unable to generate F5 ADS config name",
				err.Error(),
			)
			return
		}
		plan.Name = types.StringValue(name)
	}

	configDirs, err := configsToAPI(plan.Configs)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to parse NGINX configuration files",
			err.Error(),
		)
		return
	}
	confPath := nginxConfPath
	payloads, err := payloadsToAPI(plan.Payloads)
	if err != nil {
		resp.Diagnostics.AddError("Unable to encode F5 ADS configuration payloads", err.Error())
		return
	}
	cfgReq := configs.NginxConfigCreateRequest{
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueStringPointer(),
		Config: configs.NGINXaaSConfigRequest{
			ConfPath: &confPath,
			Configs:  configDirs,
			Payloads: payloads,
		},
	}

	created, err := r.client.CreateNginxConfigWithResponse(ctx, cfgReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create F5 ADS config",
			err.Error(),
		)
		return
	}

	if created.StatusCode() != http.StatusCreated {
		resp.Diagnostics.AddError(
			"Unable to create F5 ADS config",
			formatAPIError(created.StatusCode(), created.Body),
		)
		return
	}

	if created.JSON201 == nil {
		resp.Diagnostics.AddError(
			"Server returned empty F5 ADS config",
			"Received empty F5 ADS config object from server.",
		)
		return
	}

	setComputedAttributes(&plan, created.JSON201)
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with latest data.
func (r *configResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state configResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	found := r.fetchConfig(ctx, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if !found {
		// Reconcile Terraform state with what the API returns. If the resource was
		// deleted outside Terraform, remove it from state and let Terraform handle
		// the remaining reconciliation.
		resp.State.RemoveResource(ctx)
		return
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// fetchConfig retrieves the config from the API and populates model with its
// metadata and the contents of its latest version, using the ID already set on model.
func (r *configResource) fetchConfig(ctx context.Context, model *configResourceModel, diags *diag.Diagnostics) bool {
	configObjectID, err := objects.Parse(model.Id.ValueString())
	if err != nil {
		diags.AddError(
			"Unable to parse F5 ADS config Object ID",
			err.Error(),
		)
		return false
	}

	got, err := r.client.GetNginxConfigWithResponse(ctx, *configObjectID)
	if err != nil {
		diags.AddError(
			"Unable to read F5 ADS config",
			err.Error(),
		)
		return false
	}

	if got.StatusCode() == http.StatusNotFound {
		return false
	}

	if got.StatusCode() != http.StatusOK {
		diags.AddError(
			"Unable to read F5 ADS config",
			formatAPIError(got.StatusCode(), got.Body),
		)
		return false
	}

	if got.JSON200 == nil {
		diags.AddError(
			"Server returned empty F5 ADS config",
			"Received empty F5 ADS config object from server.",
		)
		return false
	}

	setConfigMetadata(model, got.JSON200)
	return r.fetchConfigVersion(ctx, model, diags)
}

// setConfigMetadata copies the config metadata returned by the API onto model.
func setConfigMetadata(model *configResourceModel, cfg *configs.NginxConfig) {
	model.Name = types.StringValue(cfg.Name)

	if cfg.Description != nil && *cfg.Description != "" {
		model.Description = types.StringValue(*cfg.Description)
	} else {
		model.Description = types.StringNull()
	}
	setComputedAttributes(model, cfg)
}

// setComputedAttributes sets only the values the server assigns.
func setComputedAttributes(model *configResourceModel, cfg *configs.NginxConfig) {
	model.Id = types.StringValue(cfg.ObjectId.String())
	model.OrganizationID = types.StringValue(cfg.OrganizationId.String())
	model.LatestVersionID = types.StringValue(cfg.LatestVersion.String())
}

// fetchConfigVersion retrieves the latest config version using the IDs on model
// and populates its file contents.
func (r *configResource) fetchConfigVersion(
	ctx context.Context,
	model *configResourceModel,
	diags *diag.Diagnostics,
) bool {
	configObjectID, err := objects.Parse(model.Id.ValueString())
	if err != nil {
		diags.AddError("Unable to parse F5 ADS config Object ID", err.Error())
		return false
	}

	versionID, err := objects.Parse(model.LatestVersionID.ValueString())
	if err != nil {
		diags.AddError("Unable to parse F5 ADS config version Object ID", err.Error())
		return false
	}

	version, err := r.client.GetConfigVersionWithResponse(ctx, *configObjectID, *versionID)
	if err != nil {
		diags.AddError(
			"Unable to read F5 ADS config version",
			err.Error(),
		)
		return false
	}

	if version.StatusCode() == http.StatusNotFound {
		diags.AddError(
			"F5 ADS config version not found",
			fmt.Sprintf("Latest version %q of F5 ADS config %q was not found.", versionID.String(), model.Id.ValueString()),
		)
		return false
	}

	if version.StatusCode() != http.StatusOK {
		diags.AddError(
			"Unable to read F5 ADS config version",
			formatAPIError(version.StatusCode(), version.Body),
		)
		return false
	}

	if version.JSON200 == nil {
		diags.AddError(
			"Server returned empty F5 ADS config version",
			"Received empty F5 ADS config version object from server.",
		)
		return false
	}

	model.Configs = configsFromAPI(version.JSON200.Configs)
	model.Payloads = payloadsFromAPI(version.JSON200.Payloads, model.Payloads)

	return true
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *configResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan configResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state configResourceModel
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	configObjectID, err := objects.Parse(plan.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to parse F5 ADS config Object ID",
			err.Error(),
		)
		return
	}

	configDirs, err := configsToAPI(plan.Configs)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to parse NGINX configuration files",
			err.Error(),
		)
		return
	}

	description := plan.Description.ValueStringPointer()
	if description == nil && !state.Description.IsNull() {
		empty := ""
		description = &empty
	}

	confPath := nginxConfPath
	payloads, err := payloadsToAPI(plan.Payloads)
	if err != nil {
		resp.Diagnostics.AddError("Unable to encode F5 ADS configuration payloads", err.Error())
		return
	}
	// Clear previously configured payloads when the attribute is removed.
	if payloads == nil && len(state.Payloads) > 0 {
		empty := configs.NGINXaaSConfigPayloads{}
		payloads = &empty
	}
	cfgReq := configs.NginxConfigReplaceRequest{
		Description: description,
		Config: configs.NGINXaaSConfigRequest{
			ConfPath: &confPath,
			Configs:  configDirs,
			Payloads: payloads,
		},
	}

	replaced, err := r.client.ReplaceNginxConfigWithResponse(ctx, *configObjectID, cfgReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to update F5 ADS config",
			err.Error(),
		)
		return
	}

	if replaced.StatusCode() != http.StatusOK {
		resp.Diagnostics.AddError(
			"Unable to update F5 ADS config",
			formatAPIError(replaced.StatusCode(), replaced.Body),
		)
		return
	}

	if replaced.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Server returned empty F5 ADS config",
			"Received empty F5 ADS config object from server.",
		)
		return
	}

	setComputedAttributes(&plan, replaced.JSON200)
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *configResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state configResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	configObjectID, err := objects.Parse(state.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to parse F5 ADS config Object ID",
			err.Error(),
		)
		return
	}

	deleted, err := r.client.DeleteNginxConfigWithResponse(ctx, *configObjectID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to delete F5 ADS config",
			err.Error(),
		)
		return
	}

	switch deleted.StatusCode() {
	case http.StatusNoContent, http.StatusNotFound:
	default:
		resp.Diagnostics.AddError(
			"Unable to delete F5 ADS config",
			formatAPIError(deleted.StatusCode(), deleted.Body),
		)
	}
}

// ImportState imports an existing F5 ADS config into Terraform state by its ID.
func (r *configResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// generateNameWithPrefix returns prefix followed by a "-" separator and a
// random hex suffix.
func generateNameWithPrefix(prefix string) (string, error) {
	// The suffix is "-" followed by hex characters, two per random byte.
	suffix := make([]byte, (nameSuffixLength-1)/2)
	if _, err := rand.Read(suffix); err != nil {
		return "", fmt.Errorf("unable to generate random name suffix: %w", err)
	}

	// Avoid a doubled separator when the prefix already ends with one.
	name := fmt.Sprintf("%s-%s", strings.TrimSuffix(prefix, "-"), hex.EncodeToString(suffix))

	if !nameRegex.MatchString(name) || len(name) < nameMinLength || len(name) > nameMaxLength {
		return "", fmt.Errorf(
			"generated name %q is not a valid F5 ADS configuration name: "+
				"it must be %d-%d characters and match %s",
			name, nameMinLength, nameMaxLength, nameRegex,
		)
	}

	return name, nil
}

// payloadsToAPI converts the Terraform payload models into the API request representation.
func payloadsToAPI(payloads []ConfigPayloadModel) (*configs.NGINXaaSConfigPayloads, error) {
	if payloads == nil {
		return nil, nil
	}
	result := make(configs.NGINXaaSConfigPayloads, 0, len(payloads))
	for _, payload := range payloads {
		var objectID *naas.PayloadObjectID
		if !payload.ObjectID.IsNull() && !payload.ObjectID.IsUnknown() {
			var err error
			objectID, err = objects.Parse(payload.ObjectID.ValueString())
			if err != nil {
				return nil, fmt.Errorf("unable to parse payload object_id: %w", err)
			}
		}
		paths := make([]string, 0, len(payload.Paths))
		for _, path := range payload.Paths {
			paths = append(paths, path.ValueString())
		}
		result = append(result, configs.NGINXaaSConfigPayload{
			Type:     configs.NGINXaaSConfigPayloadType(payload.Type.ValueString()),
			ObjectId: objectID,
			Paths:    paths,
		})
	}
	return &result, nil
}

// payloadsFromAPI converts the payloads returned by the API into Terraform models.
func payloadsFromAPI(payloads *configs.NGINXaaSConfigPayloads, previous []ConfigPayloadModel) []ConfigPayloadModel {
	if payloads == nil || len(*payloads) == 0 {
		if previous == nil {
			return nil
		}
		return []ConfigPayloadModel{}
	}
	result := make([]ConfigPayloadModel, 0, len(*payloads))
	for _, payload := range *payloads {
		objectID := types.StringNull()
		if payload.ObjectId != nil {
			objectID = types.StringValue(payload.ObjectId.String())
		}
		paths := make([]types.String, 0, len(payload.Paths))
		for _, path := range payload.Paths {
			paths = append(paths, types.StringValue(path))
		}
		result = append(result, ConfigPayloadModel{
			Type:     types.StringValue(string(payload.Type)),
			ObjectID: objectID,
			Paths:    paths,
		})
	}
	return result
}

// configsToAPI converts the configuration file groups from the Terraform
// model into the API request representation.
func configsToAPI(groups []ConfigGroupModel) ([]naas.DirectoryRequestWithFileContent, error) {
	dirs := make([]naas.DirectoryRequestWithFileContent, 0, len(groups))
	for _, group := range groups {
		files := make([]naas.FileDataRequest, 0, len(group.Files))
		for _, file := range group.Files {
			contents, err := base64.StdEncoding.DecodeString(file.Contents.ValueString())
			if err != nil {
				return nil, fmt.Errorf(
					"file %q in directory %q does not contain valid base64 content: %w",
					file.Name.ValueString(), group.Name.ValueString(), err,
				)
			}
			files = append(files, naas.FileDataRequest{
				Name:     file.Name.ValueString(),
				Contents: &contents,
			})
		}
		dirs = append(dirs, naas.DirectoryRequestWithFileContent{
			Name:  group.Name.ValueString(),
			Files: files,
		})
	}
	return dirs, nil
}

// configsFromAPI converts the configuration file groups returned by the API
// into the Terraform model.
func configsFromAPI(dirs []naas.DirectoryWithFileContent) []ConfigGroupModel {
	groups := make([]ConfigGroupModel, 0, len(dirs))
	for _, dir := range dirs {
		files := make([]ConfigFileModel, 0, len(dir.Files))
		for _, file := range dir.Files {
			files = append(files, ConfigFileModel{
				Name:     types.StringValue(file.Name),
				Contents: types.StringValue(base64.StdEncoding.EncodeToString(file.Contents)),
			})
		}
		groups = append(groups, ConfigGroupModel{
			Name:  types.StringValue(dir.Name),
			Files: files,
		})
	}
	return groups
}
