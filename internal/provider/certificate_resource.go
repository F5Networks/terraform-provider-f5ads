package provider

import (
	"context"
	"fmt"

	certificates "github.com/F5Networks/terraform-provider-f5ads/internal/provider/clients/certificates/2026-07-31"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
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

// type certificateResourceModel struct {
// 	Id          types.String `tfsdk:"id"`
// 	Name        types.String `tfsdk:"name"`
// 	PrivateKey  types.String `tfsdk:"private_key"`
// 	PublicCerts types.String `tfsdk:"public_certs"`
// 	Type        types.String `tfsdk:"type"`
// 	Subject     types.String `tfsdk:"subject"`
// 	Status      types.String `tfsdk:"status"`
// 	NotBefore   types.String `tfsdk:"not_before"`
// 	NotAfter    types.String `tfsdk:"not_after"`
// }

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
func (r *certificateResource) Create(ctx context.Context, _ resource.CreateRequest, _ *resource.CreateResponse) {
}

// Read refreshes the Terraform state with latest data.
func (r *certificateResource) Read(ctx context.Context, _ resource.ReadRequest, _ *resource.ReadResponse) {
}

// Update updates the terraform resource and sets it in the configuration on success.
func (r *certificateResource) Update(ctx context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
}

// Delete deletes the resource and removes it from the terraform configuration.
func (r *certificateResource) Delete(ctx context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

func (r *certificateResource) ImportState(ctx context.Context, _ resource.ImportStateRequest, _ *resource.ImportStateResponse) {
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
