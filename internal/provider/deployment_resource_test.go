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
  nginx_config_id = f5ads_configuration.deployment.id
  nginx_config_version_id = f5ads_configuration.deployment.latest_version_id
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
`, baseGcpConfig(nameSuffix), providerConfig+baseNginxConfig(nameSuffix), nameSuffix)
}

func googleCloudPrivateEndpointDeployment(nameSuffix int) string {
	return fmt.Sprintf(
		`
%s

%s

resource "f5ads_deployment" "test" {
  name = "nginxacc-%[3]d"
  capacity = 10
  nginx_config_id = f5ads_configuration.deployment.id
  nginx_config_version_id = f5ads_configuration.deployment.latest_version_id
  google_cloud_properties = {
	region = "us-east1"
	network_attachment = google_compute_network_attachment.default.id
	frontend = {
	  private_endpoint = {}
	}
  }
}
`, baseGcpConfig(nameSuffix), providerConfig+baseNginxConfig(nameSuffix), nameSuffix)
}

func googleCloudPrivateEndpointDeploymentUpdateAcceptList(nameSuffix int, gcpProject string) string {
	return fmt.Sprintf(
		`
%s

%s

resource "f5ads_deployment" "test" {
  name = "nginxacc-%[3]d"
  capacity = 10
  nginx_config_id = f5ads_configuration.deployment.id
  nginx_config_version_id = f5ads_configuration.deployment.latest_version_id
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
`, baseGcpConfig(nameSuffix), providerConfig+baseNginxConfig(nameSuffix), nameSuffix, gcpProject)
}

func googleCloudManagedPublicEndpointDeploymentUpdateCapacity(nameSuffix int) string {
	return fmt.Sprintf(
		`
%s

%s

resource "f5ads_deployment" "test" {
  name = "nginxacc-%[3]d"
  capacity = 20
  nginx_config_id = f5ads_configuration.deployment.id
  nginx_config_version_id = f5ads_configuration.deployment.latest_version_id
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
`, baseGcpConfig(nameSuffix), providerConfig+baseNginxConfig(nameSuffix), nameSuffix)
}

func googleCloudManagedPublicEndpointDeploymentUpdateIdentityAndObservability(nameSuffix int) string {
	return fmt.Sprintf(
		`
%s

%s

resource "f5ads_deployment" "test" {
  name = "nginxacc-%[3]d"
  capacity = 10
  nginx_config_id = f5ads_configuration.deployment.id
  nginx_config_version_id = f5ads_configuration.deployment.latest_version_id
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
`, baseGcpConfig(nameSuffix), providerConfig+baseNginxConfig(nameSuffix), nameSuffix)
}

func googleCloudManagedPublicEndpointDeploymentUpdateWaf(nameSuffix int) string {
	return fmt.Sprintf(
		`
%s

%s

resource "f5ads_deployment" "test" {
  name = "nginxacc-%[3]d"
  capacity = 10
  nginx_config_id = f5ads_configuration.deployment.id
  nginx_config_version_id = f5ads_configuration.deployment.latest_version_id
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
`, baseGcpConfig(nameSuffix), providerConfig+baseNginxConfig(nameSuffix), nameSuffix)
}

func TestAccDeploymentResourceGoogleManagedPublicEndpoint(t *testing.T) {
	nameSuffix := randomNameSuffix()
	gcpProject := os.Getenv("GOOGLE_PROJECT")
	resource.ParallelTest(t, resource.TestCase{
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

func awsManagedPublicEndpointDeployment(nameSuffix int) string {
	return fmt.Sprintf(
		`
%s

resource "f5ads_deployment" "test" {
  name = "nginxacc-%[2]d"
  capacity = 10
  nginx_config_id = "cfg_RXmd3U3JRcOB3GNQ0QXNSA"
  nginx_config_version_id = "cv_Sxv-NjCqTz63egfJIEy5GA"
  aws_cloud_properties = {
	region = "us-east-1"
	ipv4_cidr_block = "10.0.0.0/24"
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
`, providerConfig, nameSuffix)
}

func awsManagedPublicEndpointDeploymentUpdateCapacity(nameSuffix int) string {
	return fmt.Sprintf(
		`
%s

resource "f5ads_deployment" "test" {
  name = "nginxacc-%[2]d"
  capacity = 20
  nginx_config_id = "cfg_RXmd3U3JRcOB3GNQ0QXNSA"
  nginx_config_version_id = "cv_Sxv-NjCqTz63egfJIEy5GA"
  aws_cloud_properties = {
	region = "us-east-1"
	ipv4_cidr_block = "10.0.0.0/24"
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
`, providerConfig, nameSuffix)
}

func awsManagedPublicEndpointDeploymentUpdateWaf(nameSuffix int) string {
	return fmt.Sprintf(
		`
%s

resource "f5ads_deployment" "test" {
  name = "nginxacc-%[2]d"
  capacity = 20
  nginx_config_id = "cfg_RXmd3U3JRcOB3GNQ0QXNSA"
  nginx_config_version_id = "cv_Sxv-NjCqTz63egfJIEy5GA"
  waf_enabled = true
  aws_cloud_properties = {
	region = "us-east-1"
	ipv4_cidr_block = "10.0.0.0/24"
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
`, providerConfig, nameSuffix)
}

func TestAccDeploymentResourceAWSManagedPublicEndpoint(t *testing.T) {
	nameSuffix := randomNameSuffix()
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: awsManagedPublicEndpointDeployment(nameSuffix),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5ads_deployment.test", "name", fmt.Sprintf("nginxacc-%d", nameSuffix)),
					resource.TestCheckResourceAttr("f5ads_deployment.test", "capacity", "10"),
					resource.TestCheckNoResourceAttr("f5ads_deployment.test", "waf_enabled"),
					resource.TestCheckResourceAttr("f5ads_deployment.test", "cloud", "aws"),
					resource.TestCheckResourceAttr("f5ads_deployment.test", "aws_cloud_properties.region", "us-east-1"),
					resource.TestCheckResourceAttr("f5ads_deployment.test", "aws_cloud_properties.ipv4_cidr_block", "10.0.0.0/24"),
					resource.TestCheckResourceAttr("f5ads_deployment.test", "aws_cloud_properties.frontend.managed_public_endpoint.acl.#", "1"),
					resource.TestCheckResourceAttrSet("f5ads_deployment.test", "id"),
					resource.TestCheckResourceAttrSet("f5ads_deployment.test", "aws_cloud_properties.frontend.managed_public_endpoint.service_endpoint"),
				),
			},
			{
				ResourceName:      "f5ads_deployment.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: awsManagedPublicEndpointDeploymentUpdateCapacity(nameSuffix),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5ads_deployment.test", "name", fmt.Sprintf("nginxacc-%d", nameSuffix)),
					resource.TestCheckResourceAttr("f5ads_deployment.test", "capacity", "20"),
					resource.TestCheckResourceAttr("f5ads_deployment.test", "aws_cloud_properties.region", "us-east-1"),
					resource.TestCheckResourceAttr("f5ads_deployment.test", "aws_cloud_properties.ipv4_cidr_block", "10.0.0.0/24"),
					resource.TestCheckResourceAttr("f5ads_deployment.test", "aws_cloud_properties.frontend.managed_public_endpoint.acl.#", "1"),
				),
			},
			{
				Config: awsManagedPublicEndpointDeploymentUpdateWaf(nameSuffix),
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
	resource.ParallelTest(t, resource.TestCase{
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
  nginx_config_id = f5ads_configuration.deployment.id
  nginx_config_version_id = f5ads_configuration.deployment.latest_version_id
  google_cloud_properties = {
	region = "us-east1"
	network_attachment = google_compute_network_attachment.default.id
	frontend = {
	}
  }
}
`, baseGcpConfig(1), providerConfig+baseNginxConfig(1))
}

func googleCloudDeploymentWithBothFrontendEndpoints() string {
	return fmt.Sprintf(
		`
%s

%s

resource "f5ads_deployment" "test" {
  name = "nginxacc-with-both-frontends"
  capacity = 10
  nginx_config_id = f5ads_configuration.deployment.id
  nginx_config_version_id = f5ads_configuration.deployment.latest_version_id
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
`, baseGcpConfig(1), providerConfig+baseNginxConfig(1))
}

func deploymentWithoutCloudProperties() string {
	return fmt.Sprintf(
		`
%s

resource "f5ads_deployment" "test" {
  name                    = "nginxacc-without-cloud"
  capacity                = 10
  nginx_config_id         = "cfg_RXmd3U3JRcOB3GNQ0QXNSA"
  nginx_config_version_id = "cv_Sxv-NjCqTz63egfJIEy5GA"
}
`, providerConfig)
}

func deploymentWithBothCloudProperties() string {
	return fmt.Sprintf(
		`
%s

resource "f5ads_deployment" "test" {
  name                    = "nginxacc-with-both-clouds"
  capacity                = 10
  nginx_config_id         = "cfg_RXmd3U3JRcOB3GNQ0QXNSA"
  nginx_config_version_id = "cv_Sxv-NjCqTz63egfJIEy5GA"

  google_cloud_properties = {
    region             = "us-east1"
    network_attachment = "projects/my-project/regions/us-east1/networkAttachments/my-network-attachment"
    frontend = {
      managed_public_endpoint = {
        acl = []
      }
    }
  }

  aws_cloud_properties = {
    region          = "us-east-1"
    ipv4_cidr_block = "10.0.0.0/24"
    frontend = {
      managed_public_endpoint = {
        acl = []
      }
    }
  }
}
`, providerConfig)
}

func TestAccDeploymentResourceCloudExclusivityValidation(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      deploymentWithoutCloudProperties(),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Either google_cloud_properties or aws_cloud_properties must be specified`),
			},
			{
				Config:      deploymentWithBothCloudProperties(),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Only one of google_cloud_properties or aws_cloud_properties can be specified`),
			},
		},
	})
}

func TestAccDeploymentResourceGoogleFrontendValidation(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
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
				ExpectError: regexp.MustCompile(`Either managed_public_endpoint or private_endpoint must be specified within\s*the frontend block of google_cloud_properties`),
			},
			{
				Config:      googleCloudDeploymentWithBothFrontendEndpoints(),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Only one of managed_public_endpoint or private_endpoint can be specified\s*within the frontend block of google_cloud_properties`),
			},
		},
	})
}
