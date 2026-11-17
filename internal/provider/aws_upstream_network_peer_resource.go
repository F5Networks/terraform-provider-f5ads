package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	deployments "github.com/F5Networks/terraform-provider-f5ads/internal/provider/clients/deployments/2026-07-31"
	"github.com/F5Networks/terraform-provider-f5ads/internal/provider/objects"
	"github.com/cenkalti/backoff/v5"
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
	_ resource.Resource                = &awsUpstreamNetworkPeerResource{}
	_ resource.ResourceWithConfigure   = &awsUpstreamNetworkPeerResource{}
	_ resource.ResourceWithImportState = &awsUpstreamNetworkPeerResource{}
)

func NewAWSUpstreamNetworkPeerResource() resource.Resource {
	return &awsUpstreamNetworkPeerResource{}
}

type awsUpstreamNetworkPeerResource struct {
	client *deployments.ClientWithResponses
}

type awsUpstreamNetworkPeerModel struct {
	Status              types.String `tfsdk:"status"`
	ID                  types.String `tfsdk:"id"`
	DeploymentID        types.String `tfsdk:"deployment_id"`
	PeeringConnectionID types.String `tfsdk:"peering_connection_id"`
}

func (r *awsUpstreamNetworkPeerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_aws_upstream_network_peer"
}

func (r *awsUpstreamNetworkPeerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an AWS upstream network peer for an F5 ADS deployment.",
		Attributes: map[string]schema.Attribute{
			"status": schema.StringAttribute{
				Computed:    true,
				Description: "Current upstream connection status reported by the deployment.",
			},
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Unique identifier for the Upstream Network Peer, assigned by F5 ADS.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"deployment_id": schema.StringAttribute{
				Required:    true,
				Description: "AWS deployment whose VPC is the peer VPC. Changing this value replaces the resource.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"peering_connection_id": schema.StringAttribute{
				Required:    true,
				Description: "Single AWS VPC peering connection ID to allow. Changing this value replaces the resource.",
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

func (r *awsUpstreamNetworkPeerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *awsUpstreamNetworkPeerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data awsUpstreamNetworkPeerModel
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
	if err := r.changeAllowlist(ctx, *id, data.PeeringConnectionID.ValueString(), true); err != nil {
		resp.Diagnostics.AddError("Error allowing AWS upstream network peer", err.Error())
		return
	}
	data.ID = types.StringValue(data.DeploymentID.ValueString() + "/" + data.PeeringConnectionID.ValueString())
	data.Status = types.StringNull()
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	status, err := waitForDeploymentConnectionStatus(ctx, r.client, *id, data.PeeringConnectionID.ValueString(),
		func(dep *deployments.Deployment) *deployments.NetworkConnectionStatusList {
			return dep.Status.UpstreamConnections
		})
	if err != nil {
		resp.Diagnostics.AddError("Error waiting for AWS upstream connection status", err.Error())
		return
	}
	data.Status = status
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *awsUpstreamNetworkPeerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data awsUpstreamNetworkPeerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := objects.Parse(data.DeploymentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid deployment ID", err.Error())
		return
	}
	dep, err := getConnectionDeployment(ctx, r.client, *id)
	if err != nil {
		resp.Diagnostics.AddError("Error reading AWS upstream network peer", err.Error())
		return
	}
	if dep == nil || !slices.Contains(awsUpstreamPeeringAllowlist(dep.JSON200), data.PeeringConnectionID.ValueString()) {
		resp.State.RemoveResource(ctx)
		return
	}
	data.Status = deploymentConnectionStatus(dep.JSON200.Status.UpstreamConnections, data.PeeringConnectionID.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *awsUpstreamNetworkPeerResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Upstream network peers cannot be updated",
		"F5 ADS upstream network peers are immutable and every change should force a new resource to be created. "+
			"Please report this issue to the provider developers.",
	)
}

func (r *awsUpstreamNetworkPeerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data awsUpstreamNetworkPeerModel
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
	if err := r.changeAllowlist(ctx, *id, data.PeeringConnectionID.ValueString(), false); err != nil {
		resp.Diagnostics.AddError("Error removing AWS upstream network peer", err.Error())
		return
	}
	if err := r.waitForConnection(ctx, *id, data.PeeringConnectionID.ValueString(), false); err != nil {
		resp.Diagnostics.AddError("Error waiting for AWS upstream disconnection", err.Error())
	}
}

func (r *awsUpstreamNetworkPeerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	deploymentID, connectionID, ok := strings.Cut(req.ID, "/")
	if !ok || deploymentID == "" || connectionID == "" {
		resp.Diagnostics.AddError("Invalid import ID", "Expected nonempty deployment_id/peering_connection_id.")
		return
	}
	if _, err := objects.Parse(deploymentID); err != nil {
		resp.Diagnostics.AddError("Invalid deployment ID", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("deployment_id"), deploymentID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("peering_connection_id"), connectionID)...)
}

func awsUpstreamPeeringAllowlist(dep *deployments.Deployment) []string {
	if dep.CloudProperties.Aws == nil {
		return nil
	}
	return dep.CloudProperties.Aws.VpcPeeringConnections
}

func (r *awsUpstreamNetworkPeerResource) changeAllowlist(ctx context.Context, id objects.ID, connectionID string, add bool) error {
	return changeDeploymentConnectionAllowlist(ctx, r.client, id, add, func(dep *deployments.Deployment) (*deployments.DeploymentUpdateRequest, error) {
		if dep.CloudProperties.Aws == nil {
			if !add {
				return nil, nil
			}
			return nil, errors.New("deployment must be an AWS deployment")
		}
		current := awsUpstreamPeeringAllowlist(dep)
		if slices.Contains(current, connectionID) == add {
			return nil, nil
		}
		updated := mergeConnectionAllowlist(current, connectionID, add)
		return &deployments.DeploymentUpdateRequest{
			CloudProperties: &deployments.UpdateDeploymentCloudProperties{
				Aws: &deployments.UpdateAWSDeploymentProperties{
					VpcPeeringConnections: &updated,
				},
			},
		}, nil
	})
}

func (r *awsUpstreamNetworkPeerResource) waitForConnection(ctx context.Context, id objects.ID, connectionID string, updated bool) error {
	return waitForDeploymentConnection(ctx, r.client, id, connectionID, updated, awsUpstreamPeeringAllowlist,
		func(dep *deployments.Deployment) *deployments.NetworkConnectionStatusList {
			return dep.Status.UpstreamConnections
		}, "upstream")
}

func getConnectionDeployment(ctx context.Context, client *deployments.ClientWithResponses, id objects.ID) (*deployments.GetDeploymentResponse, error) {
	return backoff.Retry(ctx, func() (*deployments.GetDeploymentResponse, error) {
		dep, err := client.GetDeploymentWithResponse(ctx, id)
		if err != nil {
			return nil, backoff.Permanent(fmt.Errorf("unable to get deployment: %w", err))
		}
		if dep.StatusCode() == http.StatusNotFound {
			return nil, nil
		}
		if dep.StatusCode() != http.StatusOK {
			err := errors.New(formatAPIError(dep.StatusCode(), dep.Body))
			if dep.StatusCode() == http.StatusTooManyRequests || dep.StatusCode() >= http.StatusInternalServerError {
				return nil, err
			}
			return nil, backoff.Permanent(err)
		}
		if dep.JSON200 == nil {
			return nil, backoff.Permanent(errors.New("server returned empty deployment"))
		}
		return dep, nil
	}, backoff.WithMaxElapsedTime(15*time.Minute))
}

// A nil change means the owned entry already has the requested membership.
func changeDeploymentConnectionAllowlist(
	ctx context.Context,
	client *deployments.ClientWithResponses,
	id objects.ID,
	add bool,
	change func(*deployments.Deployment) (*deployments.DeploymentUpdateRequest, error),
) error {
	_, err := backoff.Retry(ctx, func() (bool, error) {
		dep, err := getConnectionDeployment(ctx, client, id)
		if err != nil {
			return false, backoff.Permanent(err)
		}
		if dep == nil {
			if !add {
				return true, nil
			}
			return false, backoff.Permanent(errors.New("deployment not found"))
		}
		body, err := change(dep.JSON200)
		if err != nil {
			return false, backoff.Permanent(err)
		}
		if body == nil {
			return true, nil
		}
		etag := dep.HTTPResponse.Header.Get("ETag")
		if etag == "" {
			return false, backoff.Permanent(errors.New("deployment GET response is missing ETag; refusing an unsafe allowlist update"))
		}
		result, err := client.UpdateDeploymentWithResponse(ctx, id, &deployments.UpdateDeploymentParams{IfMatch: &etag}, *body)
		if err != nil {
			return false, backoff.Permanent(fmt.Errorf("unable to patch deployment allowlist: %w", err))
		}
		if result.StatusCode() == http.StatusPreconditionFailed {
			// Rebuild the body from a fresh GET on every retry, never replay a stale list.
			return false, errors.New("deployment changed concurrently; retrying allowlist update")
		}
		if result.StatusCode() == http.StatusNotFound && !add {
			return true, nil
		}
		if result.StatusCode() != http.StatusAccepted {
			return false, backoff.Permanent(errors.New(formatAPIError(result.StatusCode(), result.Body)))
		}
		return true, nil
	}, backoff.WithMaxElapsedTime(15*time.Minute))
	return err
}

func mergeConnectionAllowlist(current []string, connectionID string, add bool) []string {
	updated := make([]string, 0, len(current)+1)
	for _, entry := range current {
		if entry != connectionID {
			updated = append(updated, entry)
		}
	}
	if add {
		updated = append(updated, connectionID)
	}
	return updated
}

func deploymentConnectionStatus(connections *deployments.NetworkConnectionStatusList, id string) types.String {
	if connections != nil {
		for _, connection := range *connections {
			if connection.Id == id && connection.State != "" {
				return types.StringValue(string(connection.State))
			}
		}
	}
	return types.StringNull()
}

func waitForDeploymentConnectionStatus(
	ctx context.Context,
	client *deployments.ClientWithResponses,
	id objects.ID,
	connectionID string,
	connections func(*deployments.Deployment) *deployments.NetworkConnectionStatusList,
) (types.String, error) {
	return backoff.Retry(ctx, func() (types.String, error) {
		dep, err := getConnectionDeployment(ctx, client, id)
		if err != nil {
			return types.StringNull(), backoff.Permanent(err)
		}
		if dep == nil {
			return types.StringNull(), backoff.Permanent(errors.New("deployment not found"))
		}
		status := deploymentConnectionStatus(connections(dep.JSON200), connectionID)
		if status.IsNull() {
			return status, fmt.Errorf("waiting for connection %s to have a status", connectionID)
		}
		return status, nil
	}, backoff.WithMaxElapsedTime(15*time.Minute))
}

func waitForDeploymentConnection(
	ctx context.Context,
	client *deployments.ClientWithResponses,
	id objects.ID,
	connectionID string,
	updated bool,
	allowlist func(*deployments.Deployment) []string,
	connections func(*deployments.Deployment) *deployments.NetworkConnectionStatusList,
	kind string,
) error {
	_, err := backoff.Retry(ctx, func() (bool, error) {
		dep, err := getConnectionDeployment(ctx, client, id)
		if err != nil {
			return false, backoff.Permanent(err)
		}
		if dep == nil {
			if !updated {
				return true, nil
			}
			return false, backoff.Permanent(errors.New("deployment not found"))
		}
		if slices.Contains(allowlist(dep.JSON200), connectionID) != updated {
			return false, errors.New("waiting for deployment allowlist to reflect the connection change")
		}
		if status := connections(dep.JSON200); status != nil {
			for _, connection := range *status {
				if connection.Id != connectionID {
					continue
				}
				if updated {
					switch connection.State {
					case deployments.NetworkConnectionStatusStateAccepted:
						return true, nil
					case deployments.NetworkConnectionStatusStateRejected:
						details := ""
						if connection.Details != nil {
							details = ": " + *connection.Details
						}
						return false, backoff.Permanent(fmt.Errorf("connection %s is %s%s", connectionID, connection.State, details))
					case deployments.NetworkConnectionStatusStateRemoved:
						return false, backoff.Permanent(fmt.Errorf("connection %s is removed", connectionID))
					}
				} else if connection.State == deployments.NetworkConnectionStatusStateRemoved ||
					connection.State == deployments.NetworkConnectionStatusStateRejected {
					return true, nil
				}
				return false, fmt.Errorf("waiting for connection %s (current state: %s)", connectionID, connection.State)
			}
		}
		if !updated {
			return true, nil
		}
		return false, fmt.Errorf("waiting for %s connection %s to appear", kind, connectionID)
	}, backoff.WithMaxElapsedTime(15*time.Minute))
	return err
}
