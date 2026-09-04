# Example F5 ADS Google deployment with Managed Public Endpoint.
resource "f5ads_deployment" "managed_public_endpoint" {
  name                    = "mpe-deployment"
  capacity                = 20
  waf_enabled             = true
  nginx_config_id         = "cfg_OeHBf40jQFqaRmOBhPP_bw"
  nginx_config_version_id = "cv_xFo0zPcjS2OnxH8IJ6ZJjg"

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
  name                    = "pe-deployment"
  capacity                = 20
  nginx_config_id         = "cfg_OeHBf40jQFqaRmOBhPP_bw"
  nginx_config_version_id = "cv_xFo0zPcjS2OnxH8IJ6ZJjg"

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
