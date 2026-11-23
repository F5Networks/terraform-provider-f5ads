package provider

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	deployments "github.com/F5Networks/terraform-provider-f5ads/internal/provider/clients/deployments/2026-07-31"
	"github.com/F5Networks/terraform-provider-f5ads/internal/provider/objects"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &awsPrivateEndpointConnectionResource{}
	_ resource.ResourceWithConfigure   = &awsPrivateEndpointConnectionResource{}
	_ resource.ResourceWithImportState = &awsPrivateEndpointConnectionResource{}
)

func NewAWSPrivateEndpointConnectionResource() resource.Resource {
	return &awsPrivateEndpointConnectionResource{}
}

type awsPrivateEndpointConnectionResource struct {
	client *deployments.ClientWithResponses
}

type awsPrivateEndpointConnectionModel struct {
	ID           types.String `tfsdk:"id"`
	DeploymentID types.String `tfsdk:"deployment_id"`
	EndpointID   types.String `tfsdk:"endpoint_id"`
}

func (r *awsPrivateEndpointConnectionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_aws_private_endpoint_connection"
}

func (r *awsPrivateEndpointConnectionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an AWS private endpoint connection for an F5 ADS deployment.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Unique identifier for the Private Endpoint Connection, assigned by F5 ADS",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"deployment_id": schema.StringAttribute{
				Required:    true,
				Description: "AWS deployment with a private endpoint frontend. Changing this value replaces the resource.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"endpoint_id": schema.StringAttribute{
				Required:    true,
				Description: "AWS VPC endpoint ID or 12-digit AWS account ID to allow. Changing this value replaces the resource.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *awsPrivateEndpointConnectionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	clients, ok := req.ProviderData.(*apiClients)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *apiClients, got: %T. Please report this issue to the provider.", req.ProviderData))
		return
	}
	r.client = clients.deployments
}

func (r *awsPrivateEndpointConnectionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data awsPrivateEndpointConnectionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := objects.Parse(data.DeploymentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid deployment ID", err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	if err := r.changeAllowlist(ctx, *id, data.EndpointID.ValueString(), true); err != nil {
		resp.Diagnostics.AddError("Error allowing AWS private endpoint connection", err.Error())
		return
	}
	data.ID = types.StringValue(data.DeploymentID.ValueString() + "/" + data.EndpointID.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() || !strings.HasPrefix(data.EndpointID.ValueString(), "vpce-") {
		return
	}
	if err := r.waitForConnection(ctx, *id, data.EndpointID.ValueString(), true); err != nil {
		resp.Diagnostics.AddError("Error waiting for AWS private endpoint connection acceptance", err.Error())
	}
}

func (r *awsPrivateEndpointConnectionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data awsPrivateEndpointConnectionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := objects.Parse(data.DeploymentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid deployment ID", err.Error())
		return
	}
	dep, err := r.getDeployment(ctx, *id)
	if err != nil {
		resp.Diagnostics.AddError("Error reading AWS private endpoint connection", err.Error())
		return
	}
	if dep == nil || !slices.Contains(awsPrivateEndpointAllowlist(dep.JSON200), data.EndpointID.ValueString()) {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *awsPrivateEndpointConnectionResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Private endpoint connections cannot be updated",
		"F5 ADS private endpoint connections are immutable and every change should force a new resource to be created. "+
			"Please report this issue to the provider developers.",
	)
}

func (r *awsPrivateEndpointConnectionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data awsPrivateEndpointConnectionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := objects.Parse(data.DeploymentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid deployment ID", err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	if err := r.changeAllowlist(ctx, *id, data.EndpointID.ValueString(), false); err != nil {
		resp.Diagnostics.AddError("Error removing AWS private endpoint connection", err.Error())
		return
	}
	if err := r.waitForConnection(ctx, *id, data.EndpointID.ValueString(), false); err != nil {
		resp.Diagnostics.AddError("Error waiting for AWS private endpoint disconnection", err.Error())
	}
}

func (r *awsPrivateEndpointConnectionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	deploymentID, endpointID, ok := strings.Cut(req.ID, "/")
	if !ok || deploymentID == "" || endpointID == "" {
		resp.Diagnostics.AddError("Invalid import ID", "Expected nonempty deployment_id/endpoint_id.")
		return
	}
	if _, err := objects.Parse(deploymentID); err != nil {
		resp.Diagnostics.AddError("Invalid deployment ID", err.Error())
		return
	}
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("deployment_id"), deploymentID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("endpoint_id"), endpointID)...)
}

func awsPrivateEndpointAllowlist(dep *deployments.Deployment) []string {
	if dep.CloudProperties.Aws == nil || dep.CloudProperties.Aws.Frontend.PrivateEndpoint == nil ||
		dep.CloudProperties.Aws.Frontend.PrivateEndpoint.PrivateLinkConnectionAllowList == nil {
		return nil
	}
	return *dep.CloudProperties.Aws.Frontend.PrivateEndpoint.PrivateLinkConnectionAllowList
}

func (r *awsPrivateEndpointConnectionResource) getDeployment(ctx context.Context, id objects.ID) (*deployments.GetDeploymentResponse, error) {
	return getConnectionDeployment(ctx, r.client, id)
}

func (r *awsPrivateEndpointConnectionResource) changeAllowlist(ctx context.Context, id objects.ID, endpointID string, add bool) error {
	return changeDeploymentConnectionAllowlist(ctx, r.client, id, add, func(dep *deployments.Deployment) (*deployments.DeploymentUpdateRequest, error) {
		if dep.CloudProperties.Aws == nil || dep.CloudProperties.Aws.Frontend.PrivateEndpoint == nil {
			if !add {
				return nil, nil
			}
			return nil, errors.New("deployment must have an AWS private endpoint frontend")
		}
		current := awsPrivateEndpointAllowlist(dep)
		if slices.Contains(current, endpointID) == add {
			return nil, nil
		}
		updated := mergeConnectionAllowlist(current, endpointID, add)
		return &deployments.DeploymentUpdateRequest{
			CloudProperties: &deployments.UpdateDeploymentCloudProperties{
				Aws: &deployments.UpdateAWSDeploymentProperties{
					Frontend: &deployments.UpdateAWSFrontendInfo{
						PrivateEndpoint: &deployments.AWSPrivateEndpoint{PrivateLinkConnectionAllowList: &updated},
					},
				},
			},
		}, nil
	})
}

func (r *awsPrivateEndpointConnectionResource) waitForConnection(ctx context.Context, id objects.ID, endpointID string, updated bool) error {
	return waitForDeploymentConnection(ctx, r.client, id, endpointID, updated, awsPrivateEndpointAllowlist,
		func(dep *deployments.Deployment) *deployments.NetworkConnectionStatusList {
			if !strings.HasPrefix(endpointID, "vpce-") {
				return nil
			}
			return dep.Status.FrontendConnections
		}, "frontend")
}
