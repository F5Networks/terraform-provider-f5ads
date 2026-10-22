package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

const certificateResourceName = "f5ads_certificate.test"

var tlsExternalProvider = map[string]resource.ExternalProvider{
	"tls": {
		Source:            "hashicorp/tls",
		VersionConstraint: "~> 4.0",
	},
}

// leafCertificateConfig generates a self-signed leaf certificate and key for dnsName.
func leafCertificateConfig(dnsName string) string {
	return fmt.Sprintf(`
resource "tls_private_key" "leaf" {
  algorithm = "RSA"
  rsa_bits  = 2048
}

resource "tls_self_signed_cert" "leaf" {
  private_key_pem       = tls_private_key.leaf.private_key_pem
  dns_names             = ["%[1]s"]
  validity_period_hours = 8760
  allowed_uses          = ["key_encipherment", "digital_signature", "server_auth"]

  subject {
    common_name = "%[1]s"
  }
}
`, dnsName)
}

func certKeyCertificate(name, dnsName string, createBeforeDestroy bool) string {
	lifecycle := ""
	if createBeforeDestroy {
		lifecycle = `

  lifecycle {
    create_before_destroy = true
  }`
	}

	return fmt.Sprintf(`
%s

%s

resource "f5ads_certificate" "test" {
  name         = "%s"
  public_certs = base64encode(tls_self_signed_cert.leaf.cert_pem)
  private_key  = base64encode(tls_private_key.leaf.private_key_pem)
%s
}
`, providerConfig, leafCertificateConfig(dnsName), name, lifecycle)
}

func caBundleCertificate(name, commonName string) string {
	return fmt.Sprintf(`
%s

resource "tls_private_key" "ca" {
  algorithm = "RSA"
  rsa_bits  = 2048
}

resource "tls_self_signed_cert" "ca" {
  private_key_pem       = tls_private_key.ca.private_key_pem
  is_ca_certificate     = true
  validity_period_hours = 8760
  allowed_uses          = ["cert_signing", "crl_signing"]

  subject {
    common_name = "%s"
  }
}

resource "f5ads_certificate" "test" {
  name         = "%s"
  public_certs = base64encode(tls_self_signed_cert.ca.cert_pem)
}
`, providerConfig, commonName, name)
}

func namePrefixCertificate(namePrefix, dnsName string) string {
	return fmt.Sprintf(`
%s

%s

resource "f5ads_certificate" "test" {
  name_prefix  = "%s"
  public_certs = base64encode(tls_self_signed_cert.leaf.cert_pem)
  private_key  = base64encode(tls_private_key.leaf.private_key_pem)

  lifecycle {
    create_before_destroy = true
  }
}
`, providerConfig, leafCertificateConfig(dnsName), namePrefix)
}

func TestAccCertificateResourceCertKey(t *testing.T) {
	nameSuffix := randomNameSuffix()
	name := fmt.Sprintf("nginxacc-cert-%d", nameSuffix)
	renamed := fmt.Sprintf("nginxacc-cert2-%d", nameSuffix)
	dnsName := fmt.Sprintf("acc-%d.example.com", nameSuffix)
	rotatedDNSName := fmt.Sprintf("acc-rotated-%d.example.com", nameSuffix)
	idsDiffer := statecheck.CompareValue(compare.ValuesDiffer())

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		ExternalProviders:        tlsExternalProvider,
		Steps: []resource.TestStep{
			{
				Config: certKeyCertificate(name, dnsName, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(certificateResourceName, "id"),
					resource.TestCheckResourceAttr(certificateResourceName, "name", name),
					resource.TestCheckNoResourceAttr(certificateResourceName, "name_prefix"),
					resource.TestCheckResourceAttr(certificateResourceName, "type", "cert_key"),
					resource.TestCheckResourceAttr(certificateResourceName, "subject", dnsName),
					resource.TestCheckResourceAttr(certificateResourceName, "status", "valid"),
					resource.TestCheckResourceAttrSet(certificateResourceName, "not_before"),
					resource.TestCheckResourceAttrSet(certificateResourceName, "not_after"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					idsDiffer.AddStateValue(certificateResourceName, tfjsonpath.New("id")),
				},
			},
			{
				ResourceName:      certificateResourceName,
				ImportState:       true,
				ImportStateVerify: true,
				// Certificate content is write-only on the API side and never returned.
				ImportStateVerifyIgnore: []string{"public_certs", "private_key"},
			},
			{
				// Changing the certificate content must force replacement.
				Config: certKeyCertificate(name, rotatedDNSName, false),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(certificateResourceName, plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(certificateResourceName, "name", name),
					resource.TestCheckResourceAttr(certificateResourceName, "subject", rotatedDNSName),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					idsDiffer.AddStateValue(certificateResourceName, tfjsonpath.New("id")),
				},
			},
			{
				// Changing the name must force replacement.
				Config: certKeyCertificate(renamed, rotatedDNSName, true),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(certificateResourceName, plancheck.ResourceActionCreateBeforeDestroy),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(certificateResourceName, "name", renamed),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					idsDiffer.AddStateValue(certificateResourceName, tfjsonpath.New("id")),
				},
			},
		},
	})
}

func TestAccCertificateResourceCABundle(t *testing.T) {
	nameSuffix := randomNameSuffix()
	name := fmt.Sprintf("nginxacc-ca-%d", nameSuffix)
	commonName := fmt.Sprintf("nginxacc CA %d", nameSuffix)
	rotatedCommonName := fmt.Sprintf("nginxacc CA rotated %d", nameSuffix)
	idsDiffer := statecheck.CompareValue(compare.ValuesDiffer())

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		ExternalProviders:        tlsExternalProvider,
		Steps: []resource.TestStep{
			{
				Config: caBundleCertificate(name, commonName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(certificateResourceName, "id"),
					resource.TestCheckResourceAttr(certificateResourceName, "name", name),
					resource.TestCheckNoResourceAttr(certificateResourceName, "private_key"),
					resource.TestCheckResourceAttr(certificateResourceName, "type", "ca_bundle"),
					resource.TestCheckResourceAttr(certificateResourceName, "subject", commonName),
					resource.TestCheckResourceAttr(certificateResourceName, "status", "valid"),
					resource.TestCheckResourceAttrSet(certificateResourceName, "not_before"),
					resource.TestCheckResourceAttrSet(certificateResourceName, "not_after"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					idsDiffer.AddStateValue(certificateResourceName, tfjsonpath.New("id")),
				},
			},
			{
				ResourceName:            certificateResourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"public_certs"},
			},
			{
				Config: caBundleCertificate(name, rotatedCommonName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(certificateResourceName, plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(certificateResourceName, "subject", rotatedCommonName),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					idsDiffer.AddStateValue(certificateResourceName, tfjsonpath.New("id")),
				},
			},
		},
	})
}

func TestAccCertificateResourceNamePrefix(t *testing.T) {
	nameSuffix := randomNameSuffix()
	namePrefix := "acc-"
	dnsName := fmt.Sprintf("acc-%d.example.com", nameSuffix)
	rotatedDNSName := fmt.Sprintf("acc-rotated-%d.example.com", nameSuffix)
	namePattern := regexp.MustCompile("^" + regexp.QuoteMeta(namePrefix) + `[0-9a-f]{8}$`)
	namesDiffer := statecheck.CompareValue(compare.ValuesDiffer())

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		ExternalProviders:        tlsExternalProvider,
		Steps: []resource.TestStep{
			{
				Config: namePrefixCertificate(namePrefix, dnsName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(certificateResourceName, "name_prefix", namePrefix),
					resource.TestMatchResourceAttr(certificateResourceName, "name", namePattern),
					resource.TestCheckResourceAttr(certificateResourceName, "type", "cert_key"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					namesDiffer.AddStateValue(certificateResourceName, tfjsonpath.New("name")),
				},
			},
			{
				ResourceName:            certificateResourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"public_certs", "private_key", "name_prefix"},
			},
			{
				// Rotation with create_before_destroy must produce a new, non-colliding name.
				Config: namePrefixCertificate(namePrefix, rotatedDNSName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(certificateResourceName, plancheck.ResourceActionCreateBeforeDestroy),
						plancheck.ExpectUnknownValue(certificateResourceName, tfjsonpath.New("name")),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestMatchResourceAttr(certificateResourceName, "name", namePattern),
					resource.TestCheckResourceAttr(certificateResourceName, "subject", rotatedDNSName),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					namesDiffer.AddStateValue(certificateResourceName, tfjsonpath.New("name")),
				},
			},
		},
	})
}

func TestAccCertificateResourceNameValidation(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + `
resource "f5ads_certificate" "test" {
  name         = "nginxacc-cert"
  name_prefix  = "nginxacc-"
  public_certs = "Zm9v"
}
`,
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Invalid Attribute Combination`),
			},
			{
				Config: providerConfig + `
resource "f5ads_certificate" "test" {
  public_certs = "Zm9v"
}
`,
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Missing Attribute Configuration`),
			},
		},
	})
}
