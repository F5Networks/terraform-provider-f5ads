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
