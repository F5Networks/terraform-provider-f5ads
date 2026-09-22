# Example F5 ADS NGINX configuration.

resource "f5ads_configuration" "example" {
  name = "example-config"
  description = "Example NGINX configuration managed by Terraform."

  # The provider always uses "/etc/nginx/nginx.conf" as the main NGINX
  # configuration file, so that file must exist in the configs group below.
  configs = [
    {
      name = "/etc/nginx"
      files = [
        {
          name     = "nginx.conf"
          contents = filebase64("${path.module}/nginx.conf")
        },
      ]
    },
  ]
}

# Example F5 ADS Google deployment with Managed Public Endpoint.
resource "f5ads_deployment" "managed_public_endpoint" {
  name        = "mpe-deployment"
  capacity    = 20
  waf_enabled = true

  # Reference the configuration resource instead of hardcoding IDs. Terraform
  # creates the configuration first and feeds the resulting IDs in here.
  nginx_config_id         = f5ads_configuration.example.id
  nginx_config_version_id = f5ads_configuration.example.latest_version_id

  google_cloud_properties = {
    region             = "us-east1"
    network_attachment = "projects/my-project/regions/us-east1/networkAttachments/my-network-attachment"
    log_project_id     = "my-logging-project"
    metric_project_id  = "my-metrics-project"
    identity = {
      workload_identity_pool_provider_name = "projects/my-project/locations/global/workloadIdentityPools/my-pool/providers/my-provider"
    }
    frontend = {
      managed_public_endpoint = {
        acl = [
          {
            source_prefixes = ["0.0.0.0/0"]
            port_range      = "80"
            protocol        = "tcp"
          },
          {
            source_prefixes = ["0.0.0.0/0"]
            port_range      = "443"
            protocol        = "tcp"
          },
        ]
      }
    }
  }
}

# Example F5 ADS Google deployment with Private Endpoint.
resource "f5ads_deployment" "private_endpoint" {
  name     = "pe-deployment"
  capacity = 20

  nginx_config_id         = f5ads_configuration.example.id
  nginx_config_version_id = f5ads_configuration.example.latest_version_id

  google_cloud_properties = {
    region             = "us-east1"
    network_attachment = "projects/my-project/regions/us-east1/networkAttachments/my-network-attachment"
    frontend = {
      private_endpoint = {
        service_attachment_accept_list = [
          "my-gcp-project",
        ]
      }
    }
  }
}
