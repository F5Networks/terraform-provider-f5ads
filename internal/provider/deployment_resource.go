package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	deployments "github.com/F5Networks/terraform-provider-f5ads/internal/provider/clients/deployments/2026-07-31"
	objects "github.com/F5Networks/terraform-provider-f5ads/internal/provider/objects"
	"github.com/cenkalti/backoff/v5"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure orderResource satisfies the expected interface.

var (
	_ resource.Resource                     = &deploymentResource{}
	_ resource.ResourceWithConfigure        = &deploymentResource{}
	_ resource.ResourceWithImportState      = &deploymentResource{}
	_ resource.ResourceWithConfigValidators = &deploymentResource{}
)

// NewDeploymentResource is a helper function to simplify the provider implementation.
func NewDeploymentResource() resource.Resource {
	return &deploymentResource{}
}

// deploymentResource is the resource implementation.
type deploymentResource struct {
	client *deployments.ClientWithResponses
}

type deploymentResourceModel struct {
	Id                    types.String                `tfsdk:"id"`
	Name                  types.String                `tfsdk:"name"`
	Cloud                 types.String                `tfsdk:"cloud"`
	GoogleCloudProperties *GoogleCloudPropertiesModel `tfsdk:"google_cloud_properties"`
	Capacity              types.Int64                 `tfsdk:"capacity"`
	NginxConfigID         types.String                `tfsdk:"nginx_config_id"`
	NginxConfigVersionID  types.String                `tfsdk:"nginx_config_version_id"`
	OrganizationID        types.String                `tfsdk:"organization_id"`
	WafEnabled            types.Bool                  `tfsdk:"waf_enabled"`
}

type GoogleCloudPropertiesModel struct {
	Region            types.String         `tfsdk:"region"`
	NetworkAttachment types.String         `tfsdk:"network_attachment"`
	LogProjectId      types.String         `tfsdk:"log_project_id"`
	MetricProjectId   types.String         `tfsdk:"metric_project_id"`
	Identity          *GoogleIdentityModel `tfsdk:"identity"`
	Frontend          GoogleFrontendModel  `tfsdk:"frontend"`
}

type GoogleIdentityModel struct {
	WorkloadIdentityPoolProviderName types.String `tfsdk:"workload_identity_pool_provider_name"`
	NginxaasServiceAccountUniqueId   types.String `tfsdk:"f5ads_service_account_unique_id"`
}

type GoogleFrontendModel struct {
	ManagedPublicEndpoint *ManagedPublicEndpointModel `tfsdk:"managed_public_endpoint"`
	PrivateEndpoint       *PrivateEndpointModel       `tfsdk:"private_endpoint"`
}

type ManagedPublicEndpointModel struct {
	ServiceEndpoint types.String                    `tfsdk:"service_endpoint"`
	Acl             []ManagedPublicEndpointACLModel `tfsdk:"acl"`
}

type ManagedPublicEndpointACLModel struct {
	SourcePrefixes []types.String `tfsdk:"source_prefixes"`
	PortRange      types.String   `tfsdk:"port_range"`
	Protocol       types.String   `tfsdk:"protocol"`
}

type PrivateEndpointModel struct {
	ServiceAttachment           types.String   `tfsdk:"service_attachment"`
	ServiceAttachmentAcceptList []types.String `tfsdk:"service_attachment_accept_list"`
}

// Metadata returns the name of the resource.
func (r *deploymentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_deployment"
}

// Schema defines the resource schema.
func (r *deploymentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an F5 ADS deployment. Deployments are created asynchronously; " +
			"the provider polls until the deployment reaches a ready state before completing.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Unique name for the deployment. Changing this value forces a new resource to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Unique identifier for the deployment, assigned by F5 ADS.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"cloud": schema.StringAttribute{
				Computed:    true,
				Description: "Cloud provider hosting the deployment (e.g. \"google\").",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization_id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier of the organization that owns the deployment.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"capacity": schema.Int64Attribute{
				Required:    true,
				Description: "Reserved capacity for the deployment, expressed in NCUs (NGINX Capacity Units).",
			},
			"waf_enabled": schema.BoolAttribute{
				Optional:    true,
				Description: "Enables or disables F5 WAF for NGINX. Remove this field to disable WAF.",
			},
			"nginx_config_id": schema.StringAttribute{
				Required:    true,
				Description: "Identifier of the NGINX configuration to apply to the deployment.",
			},
			"nginx_config_version_id": schema.StringAttribute{
				Required:    true,
				Description: "Identifier of the specific NGINX configuration version to apply to the deployment.",
			},
			"google_cloud_properties": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "Google Cloud-specific properties for the deployment. Required when deploying to Google Cloud.",
				Attributes: map[string]schema.Attribute{
					"region": schema.StringAttribute{
						Required:    true,
						Description: "Google Cloud region where the deployment is hosted (e.g. \"us-east1\"). Changing this value forces a new resource to be created.",
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
						},
					},
					"network_attachment": schema.StringAttribute{
						Required:    true,
						Description: "Fully-qualified resource name of the Google Cloud network attachment used by the deployment. Changing this value forces a new resource to be created.",
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
						},
					},
					"log_project_id": schema.StringAttribute{
						Optional:    true,
						Description: "Google Cloud project ID where F5 ADS logs will be exported. Remove this field to disable log exporting.",
						Validators: []validator.String{
							stringvalidator.LengthAtLeast(1),
						},
					},
					"metric_project_id": schema.StringAttribute{
						Optional:    true,
						Description: "Google Cloud project ID where F5 ADS metrics will be exported. Remove this field to disable metric exporting.",
						Validators: []validator.String{
							stringvalidator.LengthAtLeast(1),
						},
					},
					"identity": schema.SingleNestedAttribute{
						Optional:    true,
						Description: "Identity configuration for the deployment.",
						Attributes: map[string]schema.Attribute{
							"workload_identity_pool_provider_name": schema.StringAttribute{
								Optional:    true,
								Description: "Fully-qualified resource name of the Google Cloud Workload Identity Pool provider to associate with the deployment. Remove this field to disable workload identity.",
								Validators: []validator.String{
									stringvalidator.LengthAtLeast(1),
								},
							},
							"f5ads_service_account_unique_id": schema.StringAttribute{
								Computed:    true,
								Description: "Unique numeric ID of the Google Cloud service account created by F5 ADS for this deployment. Use this value in GCP IAM bindings.",
							},
						},
					},
					"frontend": schema.SingleNestedAttribute{
						Required:    true,
						Description: "Frontend networking configuration for the deployment.",
						Attributes: map[string]schema.Attribute{
							"managed_public_endpoint": schema.SingleNestedAttribute{
								Optional:    true,
								Description: "Configuration for a managed public endpoint that exposes the deployment to the internet.",
								Attributes: map[string]schema.Attribute{
									"service_endpoint": schema.StringAttribute{
										Computed:    true,
										Description: "Public DNS hostname assigned to the managed endpoint by F5 ADS.",
										PlanModifiers: []planmodifier.String{
											stringplanmodifier.UseStateForUnknown(),
										},
									},
									"acl": schema.ListNestedAttribute{
										Required:    true,
										Description: "Access control rules that restrict inbound traffic to the managed public endpoint.",
										NestedObject: schema.NestedAttributeObject{
											Attributes: map[string]schema.Attribute{
												"source_prefixes": schema.ListAttribute{
													ElementType: types.StringType,
													Required:    true,
													Description: "List of source CIDR prefixes allowed by this ACL rule.",
												},
												"port_range": schema.StringAttribute{
													Required:    true,
													Description: "Port or port range this ACL rule applies to (e.g. \"80\" or \"8080-8090\").",
												},
												"protocol": schema.StringAttribute{
													Required:    true,
													Description: "Network protocol this ACL rule applies to (e.g. \"tcp\" or \"udp\").",
												},
											},
										},
									},
								},
							},
							"private_endpoint": schema.SingleNestedAttribute{
								Optional:    true,
								Description: "Configuration for a private endpoint that exposes the deployment through Google Private Service Connect.",
								Attributes: map[string]schema.Attribute{
									"service_attachment": schema.StringAttribute{
										Computed:    true,
										Description: "Google service attachment created by F5 ADS for the private endpoint.",
										PlanModifiers: []planmodifier.String{
											stringplanmodifier.UseStateForUnknown(),
										},
									},
									"service_attachment_accept_list": schema.ListAttribute{
										ElementType: types.StringType,
										Optional:    true,
										Description: "List of Google project IDs or network URLs allowed to connect to the service attachment. Remove this field to accept all connections.",
										Validators: []validator.List{
											listvalidator.SizeAtLeast(1),
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

// Create creates the resource and sets in the terraform configuration.
func (r *deploymentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan deploymentResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Generate API request body from plan
	var depReq deployments.CreateDeploymentJSONRequestBody
	depReq.Name = plan.Name.ValueString()
	depReq.Scale.Capacity = int(plan.Capacity.ValueInt64())
	depReq.WafEnabled = plan.WafEnabled.ValueBoolPointer()
	nginxConfigId, err := objects.Parse(plan.NginxConfigID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to parse NGINX Config Object ID",
			err.Error(),
		)
		return
	}
	depReq.NginxConfigId = *nginxConfigId
	nginxConfigVersionId, err := objects.Parse(plan.NginxConfigVersionID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to parse NGINX Config Version Object ID",
			err.Error(),
		)
		return
	}
	depReq.NginxConfigVersionId = *nginxConfigVersionId

	if plan.GoogleCloudProperties != nil {
		depGoogleCloudProperties := &deployments.CreateGoogleDeploymentProperties{
			Region:            plan.GoogleCloudProperties.Region.ValueString(),
			NetworkAttachment: plan.GoogleCloudProperties.NetworkAttachment.ValueString(),
			LogProjectId:      plan.GoogleCloudProperties.LogProjectId.ValueStringPointer(),
			MetricProjectId:   plan.GoogleCloudProperties.MetricProjectId.ValueStringPointer(),
		}

		if plan.GoogleCloudProperties.Identity != nil {
			depGoogleCloudProperties.Identity = &deployments.CreateGoogleIdentity{
				WorkloadIdentityPoolProviderName: plan.GoogleCloudProperties.Identity.WorkloadIdentityPoolProviderName.ValueStringPointer(),
			}
		}

		var depFrontendManagedPublicEndpoint *deployments.CreateManagedPublicEndpoint
		var depFrontendPrivateEndpoint *deployments.UpdateGooglePrivateEndpoint
		if plan.GoogleCloudProperties.Frontend.ManagedPublicEndpoint != nil {
			depFrontendManagedPublicEndpoint = &deployments.CreateManagedPublicEndpoint{}
			acl := make([]deployments.ManagedPublicEndpointACLRule, 0)
			if aclPlan := plan.GoogleCloudProperties.Frontend.ManagedPublicEndpoint.Acl; len(aclPlan) > 0 {
				for _, rule := range aclPlan {
					sourcePrefixes := make([]string, 0, len(rule.SourcePrefixes))
					for _, prefix := range rule.SourcePrefixes {
						sourcePrefixes = append(sourcePrefixes, prefix.ValueString())
					}
					protocol := deployments.ManagedPublicEndpointACLRuleProtocol(rule.Protocol.ValueString())
					acl = append(acl, deployments.ManagedPublicEndpointACLRule{
						SourcePrefixes: sourcePrefixes,
						PortRange:      rule.PortRange.ValueStringPointer(),
						Protocol:       &protocol,
					})
				}
			}
			depFrontendManagedPublicEndpoint.Acl = acl
			depGoogleCloudProperties.Frontend.ManagedPublicEndpoint = depFrontendManagedPublicEndpoint
		}
		if plan.GoogleCloudProperties.Frontend.PrivateEndpoint != nil {
			depFrontendPrivateEndpoint = &deployments.UpdateGooglePrivateEndpoint{}
			acceptListPlan := plan.GoogleCloudProperties.Frontend.PrivateEndpoint.ServiceAttachmentAcceptList
			if len(acceptListPlan) > 0 {
				acceptList := make(deployments.GoogleServiceAttachmentAcceptList, 0, len(acceptListPlan))
				for _, item := range acceptListPlan {
					acceptList = append(acceptList, item.ValueString())
				}
				depFrontendPrivateEndpoint.ServiceAttachmentAcceptList = &acceptList
			}
			depGoogleCloudProperties.Frontend.PrivateEndpoint = depFrontendPrivateEndpoint
		}

		depReq.CloudProperties = deployments.CreateDeploymentCloudProperties{
			Google: depGoogleCloudProperties,
		}
	}

	dep, err := r.client.CreateDeploymentWithResponse(ctx, depReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create deployment",
			err.Error(),
		)
		return
	}

	if dep.StatusCode() != http.StatusAccepted {
		resp.Diagnostics.AddError(
			"Error creating deployment",
			fmt.Sprintf("status: %d", dep.StatusCode()),
		)
		return
	}

	if dep.JSON202 == nil {
		resp.Diagnostics.AddError(
			"Server returned empty deployment",
			"Received empty deployment object from server.",
		)
		return
	}

	deploymentResponse, err := waitUntilDeploymentReady(ctx, r.client, &dep.JSON202.Id)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error polling deployment creation",
			"could not poll deployment creation, unexpected err"+err.Error(),
		)
		return
	}

	// map response body to model
	plan.Id = types.StringValue(deploymentResponse.Id.String())
	plan.Cloud = types.StringValue(string(deploymentResponse.Cloud))
	plan.OrganizationID = types.StringValue(deploymentResponse.OrganizationId.String())
	if plan.GoogleCloudProperties != nil {
		if plan.GoogleCloudProperties.Frontend.ManagedPublicEndpoint != nil {
			plan.GoogleCloudProperties.Frontend.ManagedPublicEndpoint.ServiceEndpoint = types.StringValue(
				*deploymentResponse.CloudProperties.Google.Frontend.ManagedPublicEndpoint.ServiceEndpoint)
		}
		if plan.GoogleCloudProperties.Frontend.PrivateEndpoint != nil {
			if deploymentResponse.CloudProperties.Google.Frontend.PrivateEndpoint != nil {
				if deploymentResponse.CloudProperties.Google.Frontend.PrivateEndpoint.ServiceAttachment != nil {
					plan.GoogleCloudProperties.Frontend.PrivateEndpoint.ServiceAttachment = types.StringValue(
						*deploymentResponse.CloudProperties.Google.Frontend.PrivateEndpoint.ServiceAttachment)
				}
			}
		}
		if plan.GoogleCloudProperties.Identity != nil {
			plan.GoogleCloudProperties.Identity.NginxaasServiceAccountUniqueId = types.StringValue(
				*deploymentResponse.CloudProperties.Google.Identity.NginxaasServiceAccountUniqueId)
		}
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read refreshes the Terraform state with latest data.
func (r *deploymentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get the current state.
	var state deploymentResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// get deployment by id
	id := state.Id.ValueString()
	deploymentObjectID, err := objects.Parse(id)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to parse deployment Object ID",
			err.Error(),
		)
		return
	}

	deployment, err := r.client.GetDeploymentWithResponse(ctx, *deploymentObjectID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to read Deployment",
			err.Error(),
		)
		return
	}

	if deployment.StatusCode() != http.StatusOK {
		resp.Diagnostics.AddError(
			"Unable to read deployment",
			fmt.Sprintf("status: %d", deployment.StatusCode()),
		)
		return
	}

	if deployment.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Server returned empty deployment",
			"Received empty deployment object from server.",
		)
		return
	}

	// map response body to model
	deploymentResponse := deployment.JSON200
	state.Id = types.StringValue(deploymentResponse.Id.String())
	state.Name = types.StringValue(deploymentResponse.Name)
	state.Cloud = types.StringValue(string(deploymentResponse.Cloud))
	state.Capacity = types.Int64Value(int64(deploymentResponse.Scale.Capacity))
	state.NginxConfigID = types.StringValue(deploymentResponse.NginxConfigId.String())
	state.NginxConfigVersionID = types.StringValue(deploymentResponse.NginxConfigVersionId.String())
	state.OrganizationID = types.StringValue(deploymentResponse.OrganizationId.String())
	wafEnabled := types.BoolNull()
	if !state.WafEnabled.IsNull() && deploymentResponse.WafEnabled != nil {
		wafEnabled = types.BoolValue(*deploymentResponse.WafEnabled)
	}
	state.WafEnabled = wafEnabled
	if deploymentResponse.CloudProperties.Google != nil {
		var googleCloudProperties GoogleCloudPropertiesModel
		googleCloudProperties.Region = types.StringValue(deploymentResponse.CloudProperties.Google.Region)
		googleCloudProperties.NetworkAttachment = types.StringValue(deploymentResponse.CloudProperties.Google.NetworkAttachment)
		googleCloudProperties.LogProjectId = types.StringNull()
		if deploymentResponse.CloudProperties.Google.LogProjectId != nil {
			googleCloudProperties.LogProjectId = types.StringValue(*deploymentResponse.CloudProperties.Google.LogProjectId)
		}
		googleCloudProperties.MetricProjectId = types.StringNull()
		if deploymentResponse.CloudProperties.Google.MetricProjectId != nil {
			googleCloudProperties.MetricProjectId = types.StringValue(*deploymentResponse.CloudProperties.Google.MetricProjectId)
		}
		var identity *GoogleIdentityModel
		if state.GoogleCloudProperties != nil &&
			state.GoogleCloudProperties.Identity != nil &&
			deploymentResponse.CloudProperties.Google.Identity != nil {
			identity = &GoogleIdentityModel{
				NginxaasServiceAccountUniqueId:   types.StringNull(),
				WorkloadIdentityPoolProviderName: types.StringNull(),
			}
			if v := deploymentResponse.CloudProperties.Google.Identity.NginxaasServiceAccountUniqueId; v != nil {
				identity.NginxaasServiceAccountUniqueId = types.StringValue(*v)
			}
			if v := deploymentResponse.CloudProperties.Google.Identity.WorkloadIdentityPoolProviderName; v != nil {
				identity.WorkloadIdentityPoolProviderName = types.StringValue(*v)
			}
		}
		googleCloudProperties.Identity = identity

		var frontend GoogleFrontendModel
		if deploymentResponse.CloudProperties.Google.Frontend.ManagedPublicEndpoint != nil {
			var managedPublicEndpoint ManagedPublicEndpointModel
			managedPublicEndpoint.ServiceEndpoint = types.StringValue(
				*deploymentResponse.CloudProperties.Google.Frontend.ManagedPublicEndpoint.ServiceEndpoint)
			acl := make([]ManagedPublicEndpointACLModel, 0)
			if aclResp := deploymentResponse.CloudProperties.Google.Frontend.ManagedPublicEndpoint.Acl; len(aclResp) > 0 {
				for _, rule := range aclResp {
					var aclRule ManagedPublicEndpointACLModel
					sourcePrefixes := make([]types.String, 0, len(rule.SourcePrefixes))
					for _, prefix := range rule.SourcePrefixes {
						sourcePrefixes = append(sourcePrefixes, types.StringValue(prefix))
					}
					aclRule.SourcePrefixes = sourcePrefixes
					if rule.PortRange != nil {
						aclRule.PortRange = types.StringValue(*rule.PortRange)
					}
					if rule.Protocol != nil {
						aclRule.Protocol = types.StringValue(string(*rule.Protocol))
					}
					acl = append(acl, aclRule)
				}
			}
			managedPublicEndpoint.Acl = acl
			frontend.ManagedPublicEndpoint = &managedPublicEndpoint
		}
		if deploymentResponse.CloudProperties.Google.Frontend.PrivateEndpoint != nil {
			var privateEndpoint PrivateEndpointModel
			if deploymentResponse.CloudProperties.Google.Frontend.PrivateEndpoint.ServiceAttachment != nil {
				privateEndpoint.ServiceAttachment = types.StringValue(
					*deploymentResponse.CloudProperties.Google.Frontend.PrivateEndpoint.ServiceAttachment)
			}
			if acceptListResp := deploymentResponse.CloudProperties.Google.Frontend.PrivateEndpoint.ServiceAttachmentAcceptList; acceptListResp != nil && len(*acceptListResp) > 0 {
				acceptList := make([]types.String, 0, len(*acceptListResp))
				for _, item := range *acceptListResp {
					acceptList = append(acceptList, types.StringValue(item))
				}
				privateEndpoint.ServiceAttachmentAcceptList = acceptList
			}
			frontend.PrivateEndpoint = &privateEndpoint
		}
		googleCloudProperties.Frontend = frontend
		state.GoogleCloudProperties = &googleCloudProperties
	}

	// Set refreshed state.
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the terraform resource and sets it in the configuration on success.
func (r *deploymentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan deploymentResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Generate API request body from plan
	// get deployment by id
	id := plan.Id.ValueString()
	deploymentObjectID, err := objects.Parse(id)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to parse deployment Object ID",
			err.Error(),
		)
		return
	}

	nginxConfigId, err := objects.Parse(plan.NginxConfigID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to parse NGINX Config Object ID",
			err.Error(),
		)
		return
	}

	nginxConfigVersionId, err := objects.Parse(plan.NginxConfigVersionID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to parse NGINX Config Version Object ID",
			err.Error(),
		)
		return
	}

	depUpdateReq := deployments.UpdateDeploymentJSONRequestBody{
		Scale: &deployments.DeploymentScale{
			Capacity: int(plan.Capacity.ValueInt64()),
		},
		NginxConfigId:        nginxConfigId,
		NginxConfigVersionId: nginxConfigVersionId,
	}

	wafEnabled := false
	if !plan.WafEnabled.IsNull() {
		wafEnabled = plan.WafEnabled.ValueBool()
	}
	depUpdateReq.WafEnabled = &wafEnabled

	if plan.GoogleCloudProperties != nil {
		var depFrontendManagedPublicEndpoint *deployments.UpdateManagedPublicEndpoint
		if plan.GoogleCloudProperties.Frontend.ManagedPublicEndpoint != nil {
			depFrontendManagedPublicEndpoint = &deployments.UpdateManagedPublicEndpoint{}
			acl := make([]deployments.ManagedPublicEndpointACLRule, 0)
			if aclPlan := plan.GoogleCloudProperties.Frontend.ManagedPublicEndpoint.Acl; len(aclPlan) > 0 {
				acl = make([]deployments.ManagedPublicEndpointACLRule, 0, len(aclPlan))
				for _, rule := range aclPlan {
					sourcePrefixes := make([]string, 0, len(rule.SourcePrefixes))
					for _, prefix := range rule.SourcePrefixes {
						sourcePrefixes = append(sourcePrefixes, prefix.ValueString())
					}
					protocol := deployments.ManagedPublicEndpointACLRuleProtocol(rule.Protocol.ValueString())
					acl = append(acl, deployments.ManagedPublicEndpointACLRule{
						SourcePrefixes: sourcePrefixes,
						PortRange:      rule.PortRange.ValueStringPointer(),
						Protocol:       &protocol,
					})
				}
			}
			depFrontendManagedPublicEndpoint.Acl = &acl
		}
		var depFrontendPrivateEndpoint *deployments.UpdateGooglePrivateEndpoint
		if plan.GoogleCloudProperties.Frontend.PrivateEndpoint != nil {
			depFrontendPrivateEndpoint = &deployments.UpdateGooglePrivateEndpoint{}
			acceptListPlan := plan.GoogleCloudProperties.Frontend.PrivateEndpoint.ServiceAttachmentAcceptList
			acceptList := make(deployments.GoogleServiceAttachmentAcceptList, 0, len(acceptListPlan))
			for _, item := range acceptListPlan {
				acceptList = append(acceptList, item.ValueString())
			}
			depFrontendPrivateEndpoint.ServiceAttachmentAcceptList = &acceptList
		}

		emptyStr := ""
		logProjectId := &emptyStr
		if !plan.GoogleCloudProperties.LogProjectId.IsNull() {
			logProjectId = plan.GoogleCloudProperties.LogProjectId.ValueStringPointer()
		}
		metricProjectId := &emptyStr
		if !plan.GoogleCloudProperties.MetricProjectId.IsNull() {
			metricProjectId = plan.GoogleCloudProperties.MetricProjectId.ValueStringPointer()
		}
		identity := &deployments.UpdateGoogleIdentity{
			WorkloadIdentityPoolProviderName: &emptyStr,
		}
		if plan.GoogleCloudProperties.Identity != nil && !plan.GoogleCloudProperties.Identity.WorkloadIdentityPoolProviderName.IsNull() {
			identity.WorkloadIdentityPoolProviderName = plan.GoogleCloudProperties.Identity.WorkloadIdentityPoolProviderName.ValueStringPointer()
		}

		depUpdateReq.CloudProperties = &deployments.UpdateDeploymentCloudProperties{
			Google: &deployments.UpdateGoogleDeploymentProperties{
				Frontend: &deployments.UpdateGoogleFrontendInfo{
					ManagedPublicEndpoint: depFrontendManagedPublicEndpoint,
					PrivateEndpoint:       depFrontendPrivateEndpoint,
				},
				LogProjectId:    logProjectId,
				MetricProjectId: metricProjectId,
				Identity:        identity,
			},
		}
	}

	dep, err := r.client.UpdateDeploymentWithResponse(ctx, *deploymentObjectID, depUpdateReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating deployment",
			"could not update deployment, unexpected err"+err.Error(),
		)
		return
	}

	if dep.StatusCode() != http.StatusAccepted {
		resp.Diagnostics.AddError(
			"Error updating deployment",
			fmt.Sprintf("status: %d", dep.StatusCode()),
		)
		return
	}

	if dep.JSON202 == nil {
		resp.Diagnostics.AddError(
			"Server returned empty deployment",
			"Received empty deployment object from server.",
		)
		return
	}

	deploymentResponse, err := waitUntilDeploymentReady(ctx, r.client, deploymentObjectID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error polling deployment update",
			"could not poll deployment update, unexpected err"+err.Error(),
		)
		return
	}

	// map response body to model
	plan.Id = types.StringValue(deploymentResponse.Id.String())
	plan.Cloud = types.StringValue(string(deploymentResponse.Cloud))
	plan.OrganizationID = types.StringValue(deploymentResponse.OrganizationId.String())
	if plan.GoogleCloudProperties != nil {
		if plan.GoogleCloudProperties.Frontend.ManagedPublicEndpoint != nil {
			plan.GoogleCloudProperties.Frontend.ManagedPublicEndpoint.ServiceEndpoint = types.StringValue(
				*deploymentResponse.CloudProperties.Google.Frontend.ManagedPublicEndpoint.ServiceEndpoint)
		}
		if plan.GoogleCloudProperties.Frontend.PrivateEndpoint != nil {
			if deploymentResponse.CloudProperties.Google.Frontend.PrivateEndpoint != nil {
				plan.GoogleCloudProperties.Frontend.PrivateEndpoint.ServiceAttachment = types.StringValue(
					*deploymentResponse.CloudProperties.Google.Frontend.PrivateEndpoint.ServiceAttachment)
			}
		}
		if plan.GoogleCloudProperties.Identity != nil {
			plan.GoogleCloudProperties.Identity.NginxaasServiceAccountUniqueId = types.StringValue(
				*deploymentResponse.CloudProperties.Google.Identity.NginxaasServiceAccountUniqueId)
		}
	}

	plan.Capacity = types.Int64Value(int64(deploymentResponse.Scale.Capacity))
	plan.NginxConfigID = types.StringValue(deploymentResponse.NginxConfigId.String())
	plan.NginxConfigVersionID = types.StringValue(deploymentResponse.NginxConfigVersionId.String())

	// Set state
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes it from the terraform configuration.
func (r *deploymentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve state.
	var state deploymentResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// get deployment by id
	id := state.Id.ValueString()
	deploymentObjectID, err := objects.Parse(id)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to parse deployment Object ID",
			err.Error(),
		)
		return
	}

	_, err = r.client.DeleteDeploymentWithResponse(ctx, *deploymentObjectID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting deployment",
			"could not delete deployment, unexpected error: "+err.Error(),
		)
		return
	}

	err = waitUntilDeploymentDeleted(ctx, r.client, deploymentObjectID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error polling deployment deletion",
			"could not poll deployment deletion, unexpected error: "+err.Error(),
		)
		return
	}
}

func (r *deploymentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Retrieve import ID and save to id attribute
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *deploymentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Add a nil check to ensure the provider has been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*deployments.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *deployments.ClientWithResponses, got: %T. Please report this issue to the provider.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *deploymentResource) ConfigValidators(
	ctx context.Context,
) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(
			path.MatchRoot("google_cloud_properties").AtName("frontend").AtName("managed_public_endpoint"),
			path.MatchRoot("google_cloud_properties").AtName("frontend").AtName("private_endpoint"),
		),
	}
}

func waitUntilDeploymentReady(
	ctx context.Context,
	c *deployments.ClientWithResponses,
	deploymentObjectID *objects.ID,
) (deployment *deployments.Deployment, err error) {
	operation := func() (deployment *deployments.Deployment, err error) {
		resp, err := c.GetDeploymentWithResponse(ctx, *deploymentObjectID)
		if err != nil {
			return nil, backoff.Permanent(errors.New("unable to get deployment: " + err.Error()))
		}

		if resp.StatusCode() == http.StatusUnauthorized ||
			resp.StatusCode() == http.StatusForbidden ||
			resp.StatusCode() == http.StatusNotFound {
			return nil, backoff.Permanent(errors.New("response status: " + resp.Status()))
		}

		if resp.StatusCode() >= http.StatusInternalServerError {
			return nil, errors.New("server error: " + resp.Status())
		}

		if resp.StatusCode() == http.StatusTooManyRequests {
			return nil, errors.New("too many requests: " + resp.Status())
		}

		if resp.StatusCode() == http.StatusOK && resp.JSON200 == nil {
			return nil, backoff.Permanent(errors.New("server returned empty deployment"))
		}

		if resp.StatusCode() == http.StatusOK && resp.JSON200 != nil {
			deployment = resp.JSON200
			if deployment.Status.ProvisioningState.State == deployments.Failed ||
				deployment.Status.ConfigState.State == deployments.Failed {
				return nil, backoff.Permanent(errors.New("deployment failed"))
			}
			if deployment.Status.ProvisioningState.State == deployments.Suspended {
				return nil, backoff.Permanent(errors.New("deployment suspended"))
			}
			if deployment.Status.ProvisioningState.State == deployments.Deleting {
				return nil, backoff.Permanent(errors.New("deployment deleting"))
			}
			if deployment.Status.ProvisioningState.State != deployments.Ready ||
				deployment.Status.ConfigState.State != deployments.Ready {
				return nil, errors.New("deployment is not in a ready state")
			}
		}

		return deployment, nil
	}

	result, err := backoff.Retry(ctx, operation, backoff.WithMaxElapsedTime(900*time.Second))
	if err != nil {
		return nil, err
	}
	return result, nil
}

func waitUntilDeploymentDeleted(
	ctx context.Context,
	c *deployments.ClientWithResponses,
	deploymentObjectID *objects.ID,
) (err error) {
	operation := func() (_ *deployments.Deployment, _ error) {
		resp, err := c.GetDeploymentWithResponse(ctx, *deploymentObjectID)
		if err != nil {
			return nil, backoff.Permanent(errors.New("unable to get deployment: " + err.Error()))
		}

		if resp.StatusCode() == http.StatusUnauthorized ||
			resp.StatusCode() == http.StatusForbidden {
			return nil, backoff.Permanent(errors.New("response status: " + resp.Status()))
		}

		if resp.StatusCode() >= http.StatusInternalServerError {
			return nil, errors.New("server error: " + resp.Status())
		}

		if resp.StatusCode() == http.StatusTooManyRequests {
			return nil, errors.New("too many requests: " + resp.Status())
		}

		if resp.StatusCode() != http.StatusNotFound {
			return nil, errors.New("deployment still exists")
		}

		return nil, nil
	}

	_, err = backoff.Retry(ctx, operation, backoff.WithMaxElapsedTime(900*time.Second))
	if err != nil {
		return err
	}
	return nil
}
