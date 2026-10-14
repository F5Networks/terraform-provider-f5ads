package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const tlsNginxConfContents = `user nginx;
worker_processes auto;
pid /run/nginx/nginx.pid;

events {
    worker_connections 768;
}

http {
    server {
        listen 443 ssl;
        server_name localhost;

        ssl_certificate     /etc/nginx/tls/server.crt;
        ssl_certificate_key /etc/nginx/tls/server.key;

        location / {
            return 200 "Hello from F5 ADS over TLS!";
        }
    }
}
`

var payloadsTLSExternalProvider = map[string]resource.ExternalProvider{
	"tls": {
		Source:            "hashicorp/tls",
		VersionConstraint: "~> 4.0",
	},
}

// payloadsCertificateConfig defines a self-signed leaf certificate and key
// uploaded as a managed f5ads_certificate named "tls".
func payloadsCertificateConfig(nameSuffix int) string {
	return fmt.Sprintf(`
resource "tls_private_key" "payloads" {
  algorithm = "RSA"
  rsa_bits  = 2048
}

resource "tls_self_signed_cert" "payloads" {
  private_key_pem       = tls_private_key.payloads.private_key_pem
  dns_names             = ["acc-payloads-%[1]d.example.com"]
  validity_period_hours = 8760
  allowed_uses          = ["key_encipherment", "digital_signature", "server_auth"]

  subject {
    common_name = "acc-payloads-%[1]d.example.com"
  }
}

resource "f5ads_certificate" "tls" {
  name         = "nginxacc-cert-%[1]d"
  public_certs = base64encode(tls_self_signed_cert.payloads.cert_pem)
  private_key  = base64encode(tls_private_key.payloads.private_key_pem)
}
`, nameSuffix)
}

// configResourceWithPayloads attaches the managed certificate and key of
// f5ads_certificate.tls to the configuration at the given paths.
func configResourceWithPayloads(nameSuffix int, certPath, keyPath string) string {
	return fmt.Sprintf(`
%s

%s

resource "f5ads_configuration" "test" {
  name = "nginxacc-%d"
  configs = [
    {
      name = "/etc/nginx"
      files = [
        {
          name     = "nginx.conf"
          contents = "%s"
        }
      ]
    }
  ]
  payloads = [
    {
      type      = "managed_certificate"
      object_id = f5ads_certificate.tls.id
      paths     = ["%s"]
    },
    {
      type      = "managed_key"
      object_id = f5ads_certificate.tls.id
      paths     = ["%s"]
    },
  ]
}
`, providerConfig, payloadsCertificateConfig(nameSuffix), nameSuffix, b64(tlsNginxConfContents), certPath, keyPath)
}

// configResourceWithoutPayloads keeps the certificate but detaches it from the configuration.
func configResourceWithoutPayloads(nameSuffix int) string {
	return fmt.Sprintf(`
%s

%s

resource "f5ads_configuration" "test" {
  name = "nginxacc-%d"
  configs = [
    {
      name = "/etc/nginx"
      files = [
        {
          name     = "nginx.conf"
          contents = "%s"
        }
      ]
    }
  ]
}
`, providerConfig, payloadsCertificateConfig(nameSuffix), nameSuffix, b64(nginxConfContents))
}

// configResourceInvalidPayload returns a configuration with a single raw payload block.
func configResourceInvalidPayload(name, payload string) string {
	return fmt.Sprintf(`
%s

resource "f5ads_configuration" "test" {
  name = "%s"
  configs = [
    {
      name = "/etc/nginx"
      files = [
        {
          name     = "nginx.conf"
          contents = "%s"
        }
      ]
    }
  ]
  payloads = [
    %s
  ]
}
`, providerConfig, name, b64(nginxConfContents), payload)
}

// TestAccConfigResourcePayloads requires the f5ads_certificate resource to be
// registered in the provider
func TestAccConfigResourcePayloads(t *testing.T) {
	nameSuffix := randomNameSuffix()
	const resourceName = "f5ads_configuration.test"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		ExternalProviders:        payloadsTLSExternalProvider,
		Steps: []resource.TestStep{
			{
				Config: configResourceWithPayloads(nameSuffix, "/etc/nginx/tls/server.crt", "/etc/nginx/tls/server.key"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttrSet(resourceName, "latest_version_id"),
					resource.TestCheckResourceAttr(resourceName, "payloads.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "payloads.0.type", "managed_certificate"),
					resource.TestCheckResourceAttrPair(resourceName, "payloads.0.object_id", "f5ads_certificate.tls", "id"),
					resource.TestCheckResourceAttr(resourceName, "payloads.0.paths.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "payloads.0.paths.0", "/etc/nginx/tls/server.crt"),
					resource.TestCheckResourceAttr(resourceName, "payloads.1.type", "managed_key"),
					resource.TestCheckResourceAttrPair(resourceName, "payloads.1.object_id", "f5ads_certificate.tls", "id"),
					resource.TestCheckResourceAttr(resourceName, "payloads.1.paths.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "payloads.1.paths.0", "/etc/nginx/tls/server.key"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Changing payload paths is an in-place PUT update.
			{
				Config: configResourceWithPayloads(nameSuffix, "/etc/nginx/certs/site.crt", "/etc/nginx/certs/site.key"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "payloads.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "payloads.0.paths.0", "/etc/nginx/certs/site.crt"),
					resource.TestCheckResourceAttr(resourceName, "payloads.1.paths.0", "/etc/nginx/certs/site.key"),
				),
			},
			// Removing payloads detaches the certificate via PUT.
			{
				Config: configResourceWithoutPayloads(nameSuffix),
				Check:  resource.TestCheckNoResourceAttr(resourceName, "payloads.#"),
			},
		},
	})
}

func TestAccConfigResourcePayloadsValidation(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: configResourceInvalidPayload("nginxacc-badtype", `{
      type      = "inline_certificate"
      object_id = "cert_123"
      paths     = ["/etc/nginx/tls/server.crt"]
    }`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`value must be one of`),
			},
			{
				Config: configResourceInvalidPayload("nginxacc-nopaths", `{
      type      = "managed_certificate"
      object_id = "cert_123"
    }`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`attribute "paths" is required`),
			},
		},
	})
}
