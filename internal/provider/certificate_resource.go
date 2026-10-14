package provider

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	certificates "github.com/F5Networks/terraform-provider-f5ads/internal/provider/clients/certificates/2026-07-31"
	"github.com/F5Networks/terraform-provider-f5ads/internal/provider/clients/naas"
	objects "github.com/F5Networks/terraform-provider-f5ads/internal/provider/objects"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure certificateResource satisfies the expected interface.

var (
	_ resource.Resource                     = &certificateResource{}
	_ resource.ResourceWithConfigure        = &certificateResource{}
	_ resource.ResourceWithImportState      = &certificateResource{}
	_ resource.ResourceWithConfigValidators = &certificateResource{}
)

// NewCertificateResource is a helper function to simplify the provider implementation.
func NewCertificateResource() resource.Resource {
	return &certificateResource{}
}

// certificateResource is the resource implementation.
type certificateResource struct {
	client *certificates.ClientWithResponses
}

type certificateResourceModel struct {
	Id          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	NamePrefix  types.String `tfsdk:"name_prefix"`
	PrivateKey  types.String `tfsdk:"private_key"`
	PublicCerts types.String `tfsdk:"public_certs"`
	Type        types.String `tfsdk:"type"`
	Subject     types.String `tfsdk:"subject"`
	Status      types.String `tfsdk:"status"`
	NotBefore   types.String `tfsdk:"not_before"`
	NotAfter    types.String `tfsdk:"not_after"`
}

// Metadata returns the name of the resource.
func (r *certificateResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_certificate"
}

// Schema defines the resource schema.
func (r *certificateResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an F5 ADS certificate.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Unique name for the certificate. Changing this value forces a new resource to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name_prefix": schema.StringAttribute{
				Optional:    true,
				Description: "Creates a unique name beginning with the specified prefix. Conflicts with `name`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Unique identifier for the certificate, assigned by F5 ADS.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"private_key": schema.StringAttribute{
				Optional:    true,
				Description: "Base64 encoded private key associated with the certificate.",
				Sensitive:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"public_certs": schema.StringAttribute{
				Required:    true,
				Description: "Base64 encoded public certificates CA bundle or leaf certificate associated with the private key.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"type": schema.StringAttribute{
				Computed:    true,
				Description: "Type of the certificate.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"subject": schema.StringAttribute{
				Computed:    true,
				Description: "Subject of the certificate.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"status": schema.StringAttribute{
				Computed:    true,
				Description: "Current status of the certificate.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"not_before": schema.StringAttribute{
				Computed:    true,
				Description: "Not before date of the certificate.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"not_after": schema.StringAttribute{
				Computed:    true,
				Description: "Not after date of the certificate.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

// Create creates the resource and sets in the terraform configuration.
func (r *certificateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan certificateResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := plan.Name.ValueString()
	if plan.Name.IsNull() || plan.Name.IsUnknown() {
		name = prefixedUniqueName(plan.NamePrefix.ValueString())
	}

	certReq := certificates.CreateCertificateJSONRequestBody{
		Name: name,
		Content: naas.CertificateContent{
			PublicCerts: plan.PublicCerts.ValueString(),
			PrivateKey:  plan.PrivateKey.ValueStringPointer(),
		},
	}

	cert, err := r.client.CreateCertificateWithResponse(ctx, certReq)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create certificate", err.Error())
		return
	}

	// Unlike deployments, certificate creation is synchronous and returns 200.
	if cert.StatusCode() != http.StatusOK {
		resp.Diagnostics.AddError(
			"Error creating certificate",
			formatAPIError(cert.StatusCode(), cert.Body),
		)
		return
	}

	if cert.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Server returned empty certificate",
			"Received empty certificate object from server.",
		)
		return
	}

	plan.Id = types.StringValue(cert.JSON200.ObjectId.String())
	plan.Name = types.StringValue(cert.JSON200.Name)
	plan.Type = types.StringValue(string(cert.JSON200.Type))
	plan.Subject = types.StringValue(cert.JSON200.SubjectName)
	plan.Status = types.StringValue(string(cert.JSON200.Status))
	plan.NotBefore = types.StringValue(cert.JSON200.NotBefore.Format(time.RFC3339))
	plan.NotAfter = types.StringValue(cert.JSON200.NotAfter.Format(time.RFC3339))

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// Read refreshes the Terraform state with latest data.
func (r *certificateResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state certificateResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	certificateObjectID, err := objects.Parse(state.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to parse certificate Object ID", err.Error())
		return
	}

	cert, err := r.client.GetCertificateWithResponse(ctx, *certificateObjectID)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read certificate", err.Error())
		return
	}

	if cert.StatusCode() == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}

	if cert.StatusCode() != http.StatusOK {
		resp.Diagnostics.AddError(
			"Unable to read certificate",
			formatAPIError(cert.StatusCode(), cert.Body),
		)
		return
	}

	if cert.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Server returned empty certificate",
			"Received empty certificate object from server.",
		)
		return
	}

	// public_certs, private_key and name_prefix are never returned by the API, so they are kept from state.
	state.Id = types.StringValue(cert.JSON200.ObjectId.String())
	state.Name = types.StringValue(cert.JSON200.Name)
	state.Type = types.StringValue(string(cert.JSON200.Type))
	state.Subject = types.StringValue(cert.JSON200.SubjectName)
	state.Status = types.StringValue(string(cert.JSON200.Status))
	state.NotBefore = types.StringValue(cert.JSON200.NotBefore.Format(time.RFC3339))
	state.NotAfter = types.StringValue(cert.JSON200.NotAfter.Format(time.RFC3339))

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// Update is never expected to be called: every configurable attribute forces replacement.
func (r *certificateResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Certificates cannot be updated",
		"F5 ADS certificates are immutable and every change should force a new resource to be created. "+
			"Please report this issue to the provider developers.",
	)
}

// Delete deletes the resource and removes it from the terraform configuration.
func (r *certificateResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state certificateResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	certificateObjectID, err := objects.Parse(state.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to parse certificate Object ID", err.Error())
		return
	}

	cert, err := r.client.DeleteCertificateWithResponse(ctx, *certificateObjectID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting certificate",
			"could not delete certificate, unexpected error: "+err.Error(),
		)
		return
	}

	switch cert.StatusCode() {
	case http.StatusOK, http.StatusNoContent, http.StatusNotFound:
		return
	case http.StatusConflict:
		// The API message only mentions deployments, but NGINX configurations can also reference a certificate.
		resp.Diagnostics.AddError(
			"Certificate is in use",
			fmt.Sprintf(
				"Certificate %s cannot be deleted because it is still referenced by one or more deployments "+
					"and/or NGINX configurations (via configuration payloads). Remove those references before "+
					"deleting the certificate.\n\n%s",
				state.Id.ValueString(),
				formatAPIError(cert.StatusCode(), cert.Body),
			),
		)
	default:
		resp.Diagnostics.AddError(
			"Error deleting certificate",
			formatAPIError(cert.StatusCode(), cert.Body),
		)
	}
}

func (r *certificateResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *certificateResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Add a nil check to ensure the provider has been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*clients)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *clients, got: %T. Please report this issue to the provider.", req.ProviderData),
		)
		return
	}

	r.client = client.certificates
}

func (r *certificateResource) ConfigValidators(
	ctx context.Context,
) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(
			path.MatchRoot("name"),
			path.MatchRoot("name_prefix"),
		),
	}
}

// prefixedUniqueName appends a random suffix so names stay unique across create_before_destroy replacements.
func prefixedUniqueName(prefix string) string {
	random := strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	return prefix + random
}
