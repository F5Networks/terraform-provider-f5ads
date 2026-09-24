# Example F5 ADS NGINX configuration.

resource "f5ads_configuration" "example" {
  # Exactly one of "name" or "name_prefix" must be set.
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

# Example using name_prefix.

resource "f5ads_configuration" "example2" {
  name_prefix = "myconfig"
  description = "NGINX configuration that with name_prefix."

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

  lifecycle {
    create_before_destroy = true
  }
}
