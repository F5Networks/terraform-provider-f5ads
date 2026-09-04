package provider

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	deployments "github.com/F5Networks/terraform-provider-f5ads/internal/provider/clients/deployments/2026-07-31"
	objects "github.com/F5Networks/terraform-provider-f5ads/internal/provider/objects"
)

// Ensure the implementation satisfies the expected interface.
var (
	_ datasource.DataSource              = &deploymentDataSource{}
	_ datasource.DataSourceWithConfigure = &deploymentDataSource{}
)

// NewDeploymentDataSource is a helper function to simplify the provider implementation.
func NewDeploymentDataSource() datasource.DataSource {
	return &deploymentDataSource{}
}

// deploymentDataSource is the data source implementation.
type deploymentDataSource struct {
	client *deployments.ClientWithResponses
}

// deploymentDataSourceModel maps the data source schema data.
type deploymentDataSourceModel struct {
	Id                    types.String                `tfsdk:"id"`
	Name                  types.String                `tfsdk:"name"`
	Cloud                 types.String                `tfsdk:"cloud"`
	Capacity              types.Int64                 `tfsdk:"capacity"`
	NginxConfigID         types.String                `tfsdk:"nginx_config_id"`
	NginxConfigVersionID  types.String                `tfsdk:"nginx_config_version_id"`
	OrganizationID        types.String                `tfsdk:"organization_id"`
	GoogleCloudProperties *GoogleCloudPropertiesModel `tfsdk:"google_cloud_properties"`
	WafEnabled            types.Bool                  `tfsdk:"waf_enabled"`
}

// Metadata returns the data source type name.
func (d *deploymentDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_deployment"
}

// Schema defines the schema for the data source.
func (d *deploymentDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Retrieves information about an existing F5 ADS deployment by its ID.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:    true,
				Description: "Unique identifier of the deployment to look up.",
			},
			"name": schema.StringAttribute{
				Computed:    true,
				Description: "Name of the deployment.",
			},
			"cloud": schema.StringAttribute{
				Computed:    true,
				Description: "Cloud provider hosting the deployment (e.g. \"google\").",
			},
			"capacity": schema.Int64Attribute{
				Computed:    true,
				Description: "Reserved capacity for the deployment, expressed in NCUs (NGINX Capacity Units).",
			},
			"waf_enabled": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether F5 WAF for NGINX is enabled for the deployment.",
			},
			"nginx_config_id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier of the NGINX configuration applied to the deployment.",
			},
			"nginx_config_version_id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier of the specific NGINX configuration version applied to the deployment.",
			},
			"organization_id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier of the organization that owns the deployment.",
			},
			"google_cloud_properties": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "Google Cloud-specific properties for the deployment.",
				Attributes: map[string]schema.Attribute{
					"region": schema.StringAttribute{
						Computed:    true,
						Description: "Google Cloud region where the deployment is hosted (e.g. \"us-east1\").",
					},
					"network_attachment": schema.StringAttribute{
						Computed:    true,
						Description: "Fully-qualified resource name of the Google Cloud network attachment used by the deployment.",
					},
					"log_project_id": schema.StringAttribute{
						Computed:    true,
						Description: "Google Cloud project ID where F5 ADS logs will be exported. Remove this field to disable log exporting.",
					},
					"metric_project_id": schema.StringAttribute{
						Computed:    true,
						Description: "Google Cloud project ID where F5 ADS metrics will be exported. Remove this field to disable metric exporting.",
					},
					"identity": schema.SingleNestedAttribute{
						Computed:    true,
						Description: "Identity configuration for the deployment.",
						Attributes: map[string]schema.Attribute{
							"workload_identity_pool_provider_name": schema.StringAttribute{
								Computed:    true,
								Description: "Fully-qualified resource name of the Google Cloud Workload Identity Pool provider to associate with the deployment. Remove this field to disable workload identity.",
							},
							"f5ads_service_account_unique_id": schema.StringAttribute{
								Computed:    true,
								Description: "Unique numeric ID of the Google Cloud service account created by F5 ADS for this deployment. Use this value in GCP IAM bindings.",
							},
						},
					},
					"frontend": schema.SingleNestedAttribute{
						Computed:    true,
						Description: "Frontend networking configuration for the deployment.",
						Attributes: map[string]schema.Attribute{
							"managed_public_endpoint": schema.SingleNestedAttribute{
								Computed:    true,
								Description: "Configuration for a managed public endpoint that exposes the deployment to the internet.",
								Attributes: map[string]schema.Attribute{
									"service_endpoint": schema.StringAttribute{
										Computed:    true,
										Description: "Public DNS hostname assigned to the managed endpoint by F5 ADS.",
									},
									"acl": schema.ListNestedAttribute{
										Computed:    true,
										Description: "Access control rules that restrict inbound traffic to the managed public endpoint.",
										NestedObject: schema.NestedAttributeObject{
											Attributes: map[string]schema.Attribute{
												"source_prefixes": schema.ListAttribute{
													ElementType: types.StringType,
													Computed:    true,
													Description: "List of source CIDR prefixes allowed by this ACL rule.",
												},
												"port_range": schema.StringAttribute{
													Computed:    true,
													Description: "Port or port range this ACL rule applies to (e.g. \"80\" or \"8080-8090\").",
												},
												"protocol": schema.StringAttribute{
													Computed:    true,
													Description: "Network protocol this ACL rule applies to (e.g. \"tcp\" or \"udp\").",
												},
											},
										},
									},
								},
							},
							"private_endpoint": schema.SingleNestedAttribute{
								Computed:    true,
								Description: "Configuration for a private endpoint that exposes the deployment through Google Private Service Connect.",
								Attributes: map[string]schema.Attribute{
									"service_attachment": schema.StringAttribute{
										Computed:    true,
										Description: "Google Service attachment for the deployment.",
									},
									"service_attachment_accept_list": schema.ListAttribute{
										ElementType: types.StringType,
										Computed:    true,
										Description: "List of Google project IDs or network URLs allowed to connect to the service attachment.",
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

// Read refreshes the terraform state with the latest data.
func (d *deploymentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state deploymentDataSourceModel

	// read tfsdk data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
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
	deployment, err := d.client.GetDeploymentWithResponse(ctx, *deploymentObjectID)
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
	state.WafEnabled = types.BoolPointerValue(deploymentResponse.WafEnabled)
	state.NginxConfigID = types.StringValue(deploymentResponse.NginxConfigId.String())
	state.NginxConfigVersionID = types.StringValue(deploymentResponse.NginxConfigVersionId.String())
	state.OrganizationID = types.StringValue(deploymentResponse.OrganizationId.String())
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
		if deploymentResponse.CloudProperties.Google.Identity != nil {
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
			if deploymentResponse.CloudProperties.Google.Frontend.ManagedPublicEndpoint.ServiceEndpoint != nil {
				managedPublicEndpoint.ServiceEndpoint = types.StringValue(
					*deploymentResponse.CloudProperties.Google.Frontend.ManagedPublicEndpoint.ServiceEndpoint)
			}
			acl := make([]ManagedPublicEndpointACLModel, 0)
			for _, rule := range deploymentResponse.CloudProperties.Google.Frontend.ManagedPublicEndpoint.Acl {
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
			managedPublicEndpoint.Acl = acl
			frontend.ManagedPublicEndpoint = &managedPublicEndpoint
		}
		if deploymentResponse.CloudProperties.Google.Frontend.PrivateEndpoint != nil {
			var privateEndpoint PrivateEndpointModel
			if deploymentResponse.CloudProperties.Google.Frontend.PrivateEndpoint.ServiceAttachment != nil {
				privateEndpoint.ServiceAttachment = types.StringValue(
					*deploymentResponse.CloudProperties.Google.Frontend.PrivateEndpoint.ServiceAttachment)
			}
			if acceptListResp := deploymentResponse.CloudProperties.Google.Frontend.PrivateEndpoint.ServiceAttachmentAcceptList; acceptListResp != nil {
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

	// set state
	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Configure adds the provider configured client to the data source.
func (d *deploymentDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.client = client
}
