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

  # TLS-enabled nginx.conf for the example with payloads. The ssl_certificate
  # and ssl_certificate_key paths must match the payload paths below.
  nginx_tls_conf = <<-EOT
    user nginx;
    worker_processes auto;
    pid /run/nginx/nginx.pid;
    worker_rlimit_nofile 524288;
    events {
        worker_connections 150000;
    }

    http {
        server {
            listen 443 ssl backlog=2048 default_server;
            http2 on;
            server_name _;

            # The ssl_certificate and ssl_certificate_key paths must match the "payload" paths 
            # in the f5ads_configuration resource.
            ssl_certificate     /etc/nginx/tls/server.crt;
            ssl_certificate_key /etc/nginx/tls/server.key;

            location / {
                return 200 "Hello from F5 ADS over TLS!";
            }
        }
    }
  EOT
}

# Managed certificate and private key referenced by the configuration payloads.
resource "f5ads_certificate" "tls" {
  name_prefix  = "example-tls-"
  public_certs = filebase64("${path.module}/server.crt")
  private_key  = filebase64("${path.module}/server.key")

  lifecycle {
    create_before_destroy = true
  }
}

resource "f5ads_configuration" "example" {
  # Set exactly one of "name" or "name_prefix": "name" specifies a fixed name,
  # while "name_prefix" lets the provider generate a name with a random suffix.
  # Use "name_prefix" with create_before_destroy to give replacements unique
  # names, avoiding naming conflicts while linked resources are updated before
  # the old configuration is deleted.
  name = "example-config"
  description = "Example F5 ADS configuration managed by Terraform."

  # The nginx.conf must use the matching ssl_certificate and ssl_certificate_key paths.
  payloads = [
    {
      type      = "managed_certificate"
      object_id = f5ads_certificate.tls.id
      paths     = ["/etc/nginx/tls/server.crt"]
    },
    {
      type      = "managed_key"
      object_id = f5ads_certificate.tls.id
      paths     = ["/etc/nginx/tls/server.key"]
    },
  ]

  # The provider always uses "/etc/nginx/nginx.conf" as the main F5 ADS
  # configuration file, so that file must exist in the configs group below.
  configs = [
    {
      name = "/etc/nginx"
      files = [
        {
          name     = "nginx.conf"
          contents = base64encode(local.nginx_tls_conf)
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
