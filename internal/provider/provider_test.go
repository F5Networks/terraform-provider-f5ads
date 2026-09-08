package provider

import (
	"fmt"
	"math/rand"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

const (
	// providerConfig is a shared configuration to combine with the actual
	// test configuration so the f5ads client can be properly configured.
	// It is also possible to use F5ADS_ environment variables to set these values,
	// and running the tests.
	providerConfig = `
provider "f5ads" {
  geo = "us"
}
`
)

var (
	// testAccProtoV6ProviderFactories are used to instantiate a provider during acceptance
	// testing. The factory function is invoked everytime a TF CLI command is invoked  to create a
	// new provider server that the CLI can reattach to.
	testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
		"f5ads": providerserver.NewProtocol6WithError(New("test")()),
	}
)

// randomNameSuffix returns a random suffix used to uniquely name test resources.
// The F5 ADS API constrains a deployment name to at most 30 characters
// Bounding the suffix to at most 8 digits keeps every test name within the limit.
func randomNameSuffix() int {
	return rand.Intn(100000000)
}

func baseGcpConfig(nameSuffix int) string {
	return fmt.Sprintf(
		`
// Google provider is configured with environment variables, so we don't need to configure it here.
provider "google" {
	region = "us-east1"
}

resource "google_compute_network" "default" {
    name = "nginxacc-vpc-%[1]d"
    auto_create_subnetworks = false
}

resource "google_compute_subnetwork" "default" {
    name = "nginxacc-subnet-%[1]d"
    region = "us-east1"

    network = google_compute_network.default.id
    ip_cidr_range = "10.0.0.0/16"
}

resource "google_compute_network_attachment" "default" {
    name   = "nginxacc-na-%[1]d"

    subnetworks = [google_compute_subnetwork.default.id]
    connection_preference = "ACCEPT_AUTOMATIC"
}
`,
		nameSuffix,
	)
}
