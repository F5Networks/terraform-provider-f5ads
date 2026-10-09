# Example F5 ADS NGINX configuration.

resource "f5ads_configuration" "example" {
  # Set exactly one of "name" or "name_prefix": "name" specifies a fixed name,
  # while "name_prefix" lets the provider generate a name with a random suffix.
  # Use "name_prefix" with create_before_destroy to give replacements unique
  # names, avoiding naming conflicts while linked resources are updated before
  # the old configuration is deleted.
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
# Combine name_prefix with create_before_destroy so a replacement configuration
# gets a unique name and can coexist with the old one, avoiding naming conflicts
# while linked resources are updated before the old configuration is deleted.

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
