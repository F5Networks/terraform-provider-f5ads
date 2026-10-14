# Recommended: a leaf certificate and private key (type "cert_key") using name_prefix.
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

# A CA-only trust bundle (type "ca_bundle"), e.g. for ssl_trusted_certificate or
# ssl_client_certificate. No private_key is supplied.
resource "f5ads_certificate" "client_ca" {
  name_prefix  = "client-ca-"
  public_certs = filebase64("${path.module}/client-ca-bundle.pem")

  lifecycle {
    create_before_destroy = true
  }
}

# A certificate with a fixed name. Replacing it requires destroying the existing
# certificate first, so it must not be referenced by any deployment or NGINX configuration.
resource "f5ads_certificate" "fixed_name" {
  name         = "fixed-name-cert"
  public_certs = filebase64("${path.module}/example.com.crt")
  private_key  = filebase64("${path.module}/example.com.key")
}
