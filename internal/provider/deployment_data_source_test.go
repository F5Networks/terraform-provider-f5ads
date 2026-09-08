package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDeploymentDataSourceManagedPublicEndpoint(t *testing.T) {
	nameSuffix := randomNameSuffix()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		ExternalProviders: map[string]resource.ExternalProvider{
			"google": {
				Source:            "hashicorp/google",
				VersionConstraint: "~> 7.40",
			},
		},
		Steps: []resource.TestStep{
			{
				Config: googleCloudManagedPublicEndpointDeployment(nameSuffix) +
					`data "f5ads_deployment" "test" { id = f5ads_deployment.test.id }`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.f5ads_deployment.test", "name", fmt.Sprintf("nginxacc-%d", nameSuffix)),
					resource.TestCheckResourceAttrSet("f5ads_deployment.test", "id"),
					resource.TestCheckResourceAttr("data.f5ads_deployment.test", "capacity", "10"),
					resource.TestCheckResourceAttrSet("data.f5ads_deployment.test", "waf_enabled"),
					resource.TestCheckResourceAttrSet("data.f5ads_deployment.test", "organization_id"),
					resource.TestCheckResourceAttr("data.f5ads_deployment.test", "cloud", "google"),
					resource.TestCheckResourceAttrSet("data.f5ads_deployment.test", "nginx_config_id"),
					resource.TestCheckResourceAttrSet("data.f5ads_deployment.test", "nginx_config_version_id"),
					resource.TestCheckResourceAttr("data.f5ads_deployment.test", "google_cloud_properties.region", "us-east1"),
					resource.TestCheckResourceAttrSet("data.f5ads_deployment.test", "google_cloud_properties.network_attachment"),
					resource.TestCheckResourceAttrSet("data.f5ads_deployment.test", "google_cloud_properties.frontend.managed_public_endpoint.service_endpoint"),
					resource.TestCheckResourceAttr("data.f5ads_deployment.test", "google_cloud_properties.frontend.managed_public_endpoint.acl.#", "1"),
					resource.TestCheckResourceAttrSet("data.f5ads_deployment.test", "google_cloud_properties.identity.f5ads_service_account_unique_id"),
					resource.TestCheckNoResourceAttr("data.f5ads_deployment.test", "google_cloud_properties.identity.workload_identity_pool_provider_name"),
					resource.TestCheckNoResourceAttr("data.f5ads_deployment.test", "google_cloud_properties.log_project_id"),
					resource.TestCheckNoResourceAttr("data.f5ads_deployment.test", "google_cloud_properties.metric_project_id"),
				),
			},
			{
				Config: googleCloudManagedPublicEndpointDeploymentUpdateIdentityAndObservability(nameSuffix) +
					`data "f5ads_deployment" "test" { id = f5ads_deployment.test.id }`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.f5ads_deployment.test", "google_cloud_properties.identity.f5ads_service_account_unique_id"),
					resource.TestCheckResourceAttrSet("data.f5ads_deployment.test", "google_cloud_properties.identity.workload_identity_pool_provider_name"),
					resource.TestCheckResourceAttrSet("data.f5ads_deployment.test", "google_cloud_properties.log_project_id"),
					resource.TestCheckResourceAttrSet("data.f5ads_deployment.test", "google_cloud_properties.metric_project_id"),
				),
			},
		},
	})
}

func TestAccDeploymentDataSourcePrivateEndpoint(t *testing.T) {
	nameSuffix := randomNameSuffix()
	gcpProject := os.Getenv("GOOGLE_PROJECT")
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		ExternalProviders: map[string]resource.ExternalProvider{
			"google": {
				Source:            "hashicorp/google",
				VersionConstraint: "~> 7.40",
			},
		},
		Steps: []resource.TestStep{
			{
				Config: googleCloudPrivateEndpointDeployment(nameSuffix) +
					`data "f5ads_deployment" "test" { id = f5ads_deployment.test.id }`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.f5ads_deployment.test", "name", fmt.Sprintf("nginxacc-%d", nameSuffix)),
					resource.TestCheckResourceAttr("data.f5ads_deployment.test", "capacity", "10"),
					resource.TestCheckResourceAttr("data.f5ads_deployment.test", "cloud", "google"),
					resource.TestCheckResourceAttr("data.f5ads_deployment.test", "google_cloud_properties.region", "us-east1"),
					resource.TestCheckResourceAttrSet("data.f5ads_deployment.test", "google_cloud_properties.network_attachment"),
					resource.TestCheckResourceAttrSet("data.f5ads_deployment.test", "google_cloud_properties.frontend.private_endpoint.service_attachment"),
					resource.TestCheckNoResourceAttr("data.f5ads_deployment.test", "google_cloud_properties.frontend.private_endpoint.service_attachment_accept_list"),
				),
			},
			{
				Config: googleCloudPrivateEndpointDeploymentUpdateAcceptList(nameSuffix, gcpProject) +
					`data "f5ads_deployment" "test" { id = f5ads_deployment.test.id }`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.f5ads_deployment.test", "google_cloud_properties.frontend.private_endpoint.service_attachment"),
					resource.TestCheckResourceAttr("data.f5ads_deployment.test", "google_cloud_properties.frontend.private_endpoint.service_attachment_accept_list.#", "1"),
					resource.TestCheckResourceAttr("data.f5ads_deployment.test", "google_cloud_properties.frontend.private_endpoint.service_attachment_accept_list.0", gcpProject),
				),
			},
		},
	})
}
