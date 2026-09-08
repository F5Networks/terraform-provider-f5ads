package provider

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func googleCloudManagedPublicEndpointDeployment(nameSuffix int) string {
	return fmt.Sprintf(
		`
%s

%s

resource "f5ads_deployment" "test" {
  name = "nginxacc-%[3]d"
  capacity = 10
  nginx_config_id = "cfg_RXmd3U3JRcOB3GNQ0QXNSA"
  nginx_config_version_id = "cv_Sxv-NjCqTz63egfJIEy5GA"
  google_cloud_properties = {
	region = "us-east1"
	network_attachment = google_compute_network_attachment.default.id
	frontend = {
	  managed_public_endpoint = {
		acl = [
		  {
			source_prefixes = [
			  "0.0.0.0/0",
			]
			port_range = "80"
			protocol = "tcp"
		  }
		]
	  }
	}
  }
}
`, baseGcpConfig(nameSuffix), providerConfig, nameSuffix)
}

func googleCloudPrivateEndpointDeployment(nameSuffix int) string {
	return fmt.Sprintf(
		`
%s

%s

resource "f5ads_deployment" "test" {
  name = "nginxacc-%[3]d"
  capacity = 10
  nginx_config_id = "cfg_RXmd3U3JRcOB3GNQ0QXNSA"
  nginx_config_version_id = "cv_Sxv-NjCqTz63egfJIEy5GA"
  google_cloud_properties = {
	region = "us-east1"
	network_attachment = google_compute_network_attachment.default.id
	frontend = {
	  private_endpoint = {}
	}
  }
}
`, baseGcpConfig(nameSuffix), providerConfig, nameSuffix)
}

func googleCloudPrivateEndpointDeploymentUpdateAcceptList(nameSuffix int, gcpProject string) string {
	return fmt.Sprintf(
		`
%s

%s

resource "f5ads_deployment" "test" {
  name = "nginxacc-%[3]d"
  capacity = 10
  nginx_config_id = "cfg_RXmd3U3JRcOB3GNQ0QXNSA"
  nginx_config_version_id = "cv_Sxv-NjCqTz63egfJIEy5GA"
  google_cloud_properties = {
	region = "us-east1"
	network_attachment = google_compute_network_attachment.default.id
	frontend = {
	  private_endpoint = {
		service_attachment_accept_list = [
		  "%s",
		]
	  }
	}
  }
}
`, baseGcpConfig(nameSuffix), providerConfig, nameSuffix, gcpProject)
}

func googleCloudManagedPublicEndpointDeploymentUpdateCapacity(nameSuffix int) string {
	return fmt.Sprintf(
		`
%s

%s

resource "f5ads_deployment" "test" {
  name = "nginxacc-%[3]d"
  capacity = 20
  nginx_config_id = "cfg_RXmd3U3JRcOB3GNQ0QXNSA"
  nginx_config_version_id = "cv_Sxv-NjCqTz63egfJIEy5GA"
  google_cloud_properties = {
	  region = "us-east1"
	network_attachment = google_compute_network_attachment.default.id
	frontend = {
	  managed_public_endpoint = {
		acl = [
		  {
			source_prefixes = [
			  "0.0.0.0/0",
			]
			port_range = "80"
			protocol = "tcp"
		  }
		]
	  }
	}
  }
}
`, baseGcpConfig(nameSuffix), providerConfig, nameSuffix)
}

func googleCloudManagedPublicEndpointDeploymentUpdateIdentityAndObservability(nameSuffix int) string {
	return fmt.Sprintf(
		`
%s

%s

resource "f5ads_deployment" "test" {
  name = "nginxacc-%[3]d"
  capacity = 10
  nginx_config_id = "cfg_RXmd3U3JRcOB3GNQ0QXNSA"
  nginx_config_version_id = "cv_Sxv-NjCqTz63egfJIEy5GA"
  google_cloud_properties = {
	region = "us-east1"
	network_attachment = google_compute_network_attachment.default.id
	identity = {
	  workload_identity_pool_provider_name = "projects/my-project/locations/global/workloadIdentityPools/my-pool/providers/my-provider"
	}
	log_project_id = "my-log-project"
	metric_project_id = "my-metric-project"
	frontend = {
	  managed_public_endpoint = {
		acl = [
		  {
			source_prefixes = [
			  "0.0.0.0/0",
			]
			port_range = "80"
			protocol = "tcp"
		  }
		]
	  }
	}
  }
}
`, baseGcpConfig(nameSuffix), providerConfig, nameSuffix)
}

func googleCloudManagedPublicEndpointDeploymentUpdateWaf(nameSuffix int) string {
	return fmt.Sprintf(
		`
%s

%s

resource "f5ads_deployment" "test" {
  name = "nginxacc-%[3]d"
  capacity = 10
  nginx_config_id = "cfg_RXmd3U3JRcOB3GNQ0QXNSA"
  nginx_config_version_id = "cv_Sxv-NjCqTz63egfJIEy5GA"
  waf_enabled = true
  google_cloud_properties = {
	region = "us-east1"
	network_attachment = google_compute_network_attachment.default.id
	identity = {
	  workload_identity_pool_provider_name = "projects/my-project/locations/global/workloadIdentityPools/my-pool/providers/my-provider"
	}
	log_project_id = "my-log-project"
	metric_project_id = "my-metric-project"
	frontend = {
	  managed_public_endpoint = {
		acl = [
		  {
			source_prefixes = [
			  "0.0.0.0/0",
			]
			port_range = "80"
			protocol = "tcp"
		  }
		]
	  }
	}
  }
}
`, baseGcpConfig(nameSuffix), providerConfig, nameSuffix)
}

func TestAccDeploymentResourceGoogleManagedPublicEndpoint(t *testing.T) {
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
				Config: googleCloudManagedPublicEndpointDeployment(nameSuffix),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5ads_deployment.test", "name", fmt.Sprintf("nginxacc-%d", nameSuffix)),
					resource.TestCheckResourceAttr("f5ads_deployment.test", "capacity", "10"),
					resource.TestCheckNoResourceAttr("f5ads_deployment.test", "waf_enabled"),
					resource.TestCheckResourceAttr("f5ads_deployment.test", "google_cloud_properties.region", "us-east1"),
					resource.TestCheckResourceAttr(
						"f5ads_deployment.test",
						"google_cloud_properties.network_attachment",
						fmt.Sprintf("projects/%s/regions/us-east1/networkAttachments/nginxacc-na-%d", gcpProject, nameSuffix),
					),
					resource.TestCheckResourceAttr("f5ads_deployment.test", "google_cloud_properties.frontend.managed_public_endpoint.acl.#", "1"),
					resource.TestCheckResourceAttrSet("f5ads_deployment.test", "id"),
					resource.TestCheckResourceAttrSet("f5ads_deployment.test", "google_cloud_properties.frontend.managed_public_endpoint.service_endpoint"),
				),
			},
			{
				ResourceName:      "f5ads_deployment.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: googleCloudManagedPublicEndpointDeploymentUpdateCapacity(nameSuffix),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5ads_deployment.test", "name", fmt.Sprintf("nginxacc-%d", nameSuffix)),
					resource.TestCheckResourceAttr("f5ads_deployment.test", "capacity", "20"),
					resource.TestCheckResourceAttr("f5ads_deployment.test", "google_cloud_properties.region", "us-east1"),
					resource.TestCheckResourceAttr(
						"f5ads_deployment.test",
						"google_cloud_properties.network_attachment",
						fmt.Sprintf("projects/%s/regions/us-east1/networkAttachments/nginxacc-na-%d", gcpProject, nameSuffix),
					),
					resource.TestCheckResourceAttr("f5ads_deployment.test", "google_cloud_properties.frontend.managed_public_endpoint.acl.#", "1"),
				),
			},
			{
				Config: googleCloudManagedPublicEndpointDeploymentUpdateIdentityAndObservability(nameSuffix),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5ads_deployment.test", "name", fmt.Sprintf("nginxacc-%d", nameSuffix)),
					resource.TestCheckResourceAttr("f5ads_deployment.test", "google_cloud_properties.identity.workload_identity_pool_provider_name", "projects/my-project/locations/global/workloadIdentityPools/my-pool/providers/my-provider"),
					resource.TestCheckResourceAttrSet("f5ads_deployment.test", "google_cloud_properties.identity.f5ads_service_account_unique_id"),
					resource.TestCheckResourceAttr("f5ads_deployment.test", "google_cloud_properties.log_project_id", "my-log-project"),
					resource.TestCheckResourceAttr("f5ads_deployment.test", "google_cloud_properties.metric_project_id", "my-metric-project"),
					resource.TestCheckResourceAttr(
						"f5ads_deployment.test",
						"google_cloud_properties.network_attachment",
						fmt.Sprintf("projects/%s/regions/us-east1/networkAttachments/nginxacc-na-%d", gcpProject, nameSuffix),
					),
					resource.TestCheckResourceAttr("f5ads_deployment.test", "google_cloud_properties.frontend.managed_public_endpoint.acl.#", "1"),
				),
			},
			{
				Config: googleCloudManagedPublicEndpointDeploymentUpdateWaf(nameSuffix),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5ads_deployment.test", "waf_enabled", "true"),
				),
			},
		},
	})
}

func TestAccDeploymentResourceGooglePrivateEndpoint(t *testing.T) {
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
				Config: googleCloudPrivateEndpointDeployment(nameSuffix),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5ads_deployment.test", "name", fmt.Sprintf("nginxacc-%d", nameSuffix)),
					resource.TestCheckResourceAttrSet("f5ads_deployment.test", "google_cloud_properties.frontend.private_endpoint.service_attachment"),
					resource.TestCheckNoResourceAttr("f5ads_deployment.test", "google_cloud_properties.frontend.private_endpoint.service_attachment_accept_list"),
				),
			},
			{
				ResourceName:      "f5ads_deployment.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: googleCloudPrivateEndpointDeploymentUpdateAcceptList(nameSuffix, gcpProject),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5ads_deployment.test", "google_cloud_properties.frontend.private_endpoint.service_attachment_accept_list.#", "1"),
					resource.TestCheckResourceAttr("f5ads_deployment.test", "google_cloud_properties.frontend.private_endpoint.service_attachment_accept_list.0", gcpProject),
				),
			},
		},
	})
}

func googleCloudDeploymentWithoutFrontendEndpoint() string {
	return fmt.Sprintf(
		`
%s

%s

resource "f5ads_deployment" "test" {
  name = "nginxacc-without-frontend"
  capacity = 10
  nginx_config_id = "cfg_RXmd3U3JRcOB3GNQ0QXNSA"
  nginx_config_version_id = "cv_Sxv-NjCqTz63egfJIEy5GA"
  google_cloud_properties = {
	region = "us-east1"
	network_attachment = google_compute_network_attachment.default.id
	frontend = {
	}
  }
}
`, baseGcpConfig(1), providerConfig)
}

func googleCloudDeploymentWithBothFrontendEndpoints() string {
	return fmt.Sprintf(
		`
%s

%s

resource "f5ads_deployment" "test" {
  name = "nginxacc-with-both-frontends"
  capacity = 10
  nginx_config_id = "cfg_RXmd3U3JRcOB3GNQ0QXNSA"
  nginx_config_version_id = "cv_Sxv-NjCqTz63egfJIEy5GA"
  google_cloud_properties = {
	region = "us-east1"
	network_attachment = google_compute_network_attachment.default.id
	frontend = {
	  managed_public_endpoint = {
		acl = []
	  }
	  private_endpoint = {}
	}
  }
}
`, baseGcpConfig(1), providerConfig)
}

func TestAccDeploymentResourceGoogleFrontendValidation(t *testing.T) {
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
				Config:      googleCloudDeploymentWithoutFrontendEndpoint(),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Exactly one of these attributes must be configured:\s*\[google_cloud_properties\.frontend\.managed_public_endpoint,google_cloud_properties\.frontend\.private_endpoint\]`),
			},
			{
				Config:      googleCloudDeploymentWithBothFrontendEndpoints(),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Exactly one of these attributes must be configured:\s*\[google_cloud_properties\.frontend\.managed_public_endpoint,google_cloud_properties\.frontend\.private_endpoint\]`),
			},
		},
	})
}
