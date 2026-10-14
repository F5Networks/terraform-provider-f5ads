# Certificates are immutable, so any change forces replacement. name_prefix together with
# create_before_destroy lets the new certificate be created before the old one is removed.
resource "f5ads_certificate" "example" {
  name_prefix  = "example-com-"
  public_certs = filebase64("${path.module}/example.com.crt")
  private_key  = filebase64("${path.module}/example.com.key")

  lifecycle {
    create_before_destroy = true
  }
}

# A CA-only trust bundle (type "ca_bundle") for NGINX's ssl_trusted_certificate
# or ssl_client_certificate directives. No private_key is supplied.
resource "f5ads_certificate" "client_ca" {
  name_prefix  = "client-ca-"
  public_certs = filebase64("${path.module}/client-ca-bundle.pem")

  lifecycle {
    create_before_destroy = true
  }
}

# Changing the name creates a replacement with a different unique name, so
# create_before_destroy can be used. Changing contents under the same name cannot
# be replaced this way, and destroy-before-create fails while the old certificate
# is referenced. To rotate contents, add a certificate with a new name or name_prefix,
# update references, apply, and then remove the old certificate from Terraform.
resource "f5ads_certificate" "fixed_name" {
  name         = "fixed-name-cert"
  public_certs = filebase64("${path.module}/example.com.crt")
  private_key  = filebase64("${path.module}/example.com.key")

  lifecycle {
    create_before_destroy = true
  }
}
