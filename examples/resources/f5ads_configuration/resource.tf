# Example F5 ADS configuration.

locals {
  nginx_conf = <<-EOT
    user nginx;
    worker_processes auto;
    pid /run/nginx/nginx.pid;
    worker_rlimit_nofile 524288;
    events {
        worker_connections 150000;
    }

    http {
        server {
            listen 80 backlog=2048 default_server;
            http2 on;
            server_name _;

            location / {
                return 200 "Hello from F5 ADS!";
            }
        }
    }
  EOT
}

resource "f5ads_configuration" "example" {
  # Set exactly one of "name" or "name_prefix": "name" specifies a fixed name,
  # while "name_prefix" lets the provider generate a name with a random suffix.
  # Use "name_prefix" with create_before_destroy to give replacements unique
  # names, avoiding naming conflicts while linked resources are updated before
  # the old configuration is deleted.
  name = "example-config"
  description = "Example F5 ADS configuration managed by Terraform."

  # The provider always uses "/etc/nginx/nginx.conf" as the main F5 ADS
  # configuration file, so that file must exist in the configs group below.
  configs = [
    {
      name = "/etc/nginx"
      files = [
        {
          name     = "nginx.conf"
          contents = base64encode(local.nginx_conf)
        },
      ]
    },
  ]
}

# Example using name_prefix.
# Combine name_prefix with create_before_destroy so a replacement configuration
# gets a unique name and can coexist with the old one, avoiding naming conflicts
# while linked resources are updated before the old configuration is deleted.

resource "f5ads_configuration" "example2" {
  name_prefix = "myconfig"
  description = "NGINX configuration with name_prefix."

  configs = [
    {
      name = "/etc/nginx"
      files = [
        {
          name     = "nginx.conf"
          contents = base64encode(local.nginx_conf)
        },
      ]
    },
  ]

  lifecycle {
    create_before_destroy = true
  }
}
