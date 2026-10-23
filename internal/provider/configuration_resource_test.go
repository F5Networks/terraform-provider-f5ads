package provider

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const nginxConfContents = `user nginx;
worker_processes auto;
pid /run/nginx/nginx.pid;
worker_rlimit_nofile 524288;
events {
    worker_connections 150000;
}

http {
    include /etc/nginx/conf.d/*.conf;

    server {
        listen 80 backlog=2048 default_server;
        http2 on;
        server_name _;

        location / {
            return 200 "Hello from F5 ADS!";
        }
    }
}
`

const updatedNginxConfContents = `user nginx;
worker_processes auto;
pid /run/nginx/nginx.pid;
worker_rlimit_nofile 524288;
events {
    worker_connections 150000;
}

http {
    include /etc/nginx/conf.d/*.conf;

    server {
        listen 80 backlog=2048 default_server;
        http2 on;
        server_name _;

        location / {
            return 200 "Hello!";
        }
    }
}
`

const siteConfContents = `server {
    listen 8080;
    server_name site.localhost;

    location / {
        return 200 "Hello from site.conf!";
    }
}
`

// baseNginxConfig returns an f5ads_configuration resource that deployments
// can reference instead of relying on a pre-existing configuration.
func baseNginxConfig() string {
	return fmt.Sprintf(
		`
resource "f5ads_configuration" "test" {
  name_prefix = "nginxacc-dep"
  configs = [
    {
      name = "/etc/nginx"
      files = [
        {
          name     = "nginx.conf"
          contents = base64encode(%q)
        }
      ]
    }
  ]
}
`,
		nginxConfContents,
	)
}

// configResourceWithName returns a configuration with a single nginx.conf file
// and an explicitly set name.
func configResourceWithName(nameSuffix int, description string, conf string) string {
	return fmt.Sprintf(`
%s

resource "f5ads_configuration" "test" {
  name        = "nginxacc-%d"
  description = "%s"
  configs = [
    {
      name = "/etc/nginx"
      files = [
        {
          name     = "nginx.conf"
          contents = base64encode(%q)
        }
      ]
    }
  ]
}
`, providerConfig, nameSuffix, description, conf)
}

// configResourceTwoGroups adds a second directory group with an extra file.
func configResourceTwoGroups(nameSuffix int, description string) string {
	return fmt.Sprintf(`
%s

resource "f5ads_configuration" "test" {
  name        = "nginxacc-%d"
  description = "%s"
  configs = [
    {
      name = "/etc/nginx"
      files = [
        {
          name     = "nginx.conf"
          contents = base64encode(%q)
        }
      ]
    },
    {
      name = "/etc/nginx/conf.d"
      files = [
        {
          name     = "site.conf"
          contents = base64encode(%q)
        }
      ]
    }
  ]
}
`, providerConfig, nameSuffix, description, nginxConfContents, siteConfContents)
}

// configResourceWithNamePrefix uses name_prefix.
func configResourceWithNamePrefix(prefix string) string {
	return fmt.Sprintf(`
%s

resource "f5ads_configuration" "test" {
  name_prefix = "%s"
  configs = [
    {
      name = "/etc/nginx"
      files = [
        {
          name     = "nginx.conf"
          contents = base64encode(%q)
        }
      ]
    }
  ]
}
`, providerConfig, prefix, nginxConfContents)
}

// configResourceNoName sets neither name nor name_prefix.
func configResourceNoName() string {
	return fmt.Sprintf(`
%s

resource "f5ads_configuration" "test" {
  configs = [
    {
      name = "/etc/nginx"
      files = [
        {
          name     = "nginx.conf"
          contents = base64encode(%q)
        }
      ]
    }
  ]
}
`, providerConfig, nginxConfContents)
}

// configResourceBothNames sets both name and name_prefix.
func configResourceBothNames() string {
	return fmt.Sprintf(`
%s

resource "f5ads_configuration" "test" {
  name        = "nginxacc-both"
  name_prefix = "nginxacc"
  configs = [
    {
      name = "/etc/nginx"
      files = [
        {
          name     = "nginx.conf"
          contents = base64encode(%q)
        }
      ]
    }
  ]
}
`, providerConfig, nginxConfContents)
}

// configResourceInvalidName uses a name that violates the naming rules.
func configResourceInvalidName() string {
	return fmt.Sprintf(`
%s

resource "f5ads_configuration" "test" {
  name = "Invalid_Name"
  configs = [
    {
      name = "/etc/nginx"
      files = [
        {
          name     = "nginx.conf"
          contents = base64encode(%q)
        }
      ]
    }
  ]
}
`, providerConfig, nginxConfContents)
}

// configResourceEmptyConfigs sets an empty configs set.
func configResourceEmptyConfigs() string {
	return fmt.Sprintf(`
%s

resource "f5ads_configuration" "test" {
  name    = "nginxacc-empty"
  configs = []
}
`, providerConfig)
}

func TestAccConfigResourceWithName(t *testing.T) {
	nameSuffix := randomNameSuffix()
	name := fmt.Sprintf("nginxacc-%d", nameSuffix)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: configResourceWithName(nameSuffix, "initial description", nginxConfContents),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5ads_configuration.test", "name", name),
					resource.TestCheckResourceAttr("f5ads_configuration.test", "description", "initial description"),
					resource.TestCheckResourceAttrSet("f5ads_configuration.test", "id"),
					resource.TestCheckResourceAttrSet("f5ads_configuration.test", "organization_id"),
					resource.TestCheckResourceAttrSet("f5ads_configuration.test", "latest_version_id"),
					resource.TestCheckResourceAttr("f5ads_configuration.test", "configs.#", "1"),
					resource.TestCheckResourceAttr("f5ads_configuration.test", "configs.0.name", "/etc/nginx"),
					resource.TestCheckResourceAttr("f5ads_configuration.test", "configs.0.files.#", "1"),
					resource.TestCheckResourceAttr("f5ads_configuration.test", "configs.0.files.0.name", "nginx.conf"),
					resource.TestCheckResourceAttr(
						"f5ads_configuration.test", "configs.0.files.0.contents", base64.StdEncoding.EncodeToString([]byte(nginxConfContents)),
					),
				),
			},
			{
				ResourceName:      "f5ads_configuration.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update the description only.
			{
				Config: configResourceWithName(nameSuffix, "updated description", nginxConfContents),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5ads_configuration.test", "name", name),
					resource.TestCheckResourceAttr("f5ads_configuration.test", "description", "updated description"),
					resource.TestCheckResourceAttrSet("f5ads_configuration.test", "latest_version_id"),
				),
			},
			// Update the file contents, which creates a new version.
			{
				Config: configResourceWithName(nameSuffix, "updated description", updatedNginxConfContents),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(
						"f5ads_configuration.test", "configs.0.files.0.contents", base64.StdEncoding.EncodeToString([]byte(updatedNginxConfContents)),
					),
					resource.TestCheckResourceAttrSet("f5ads_configuration.test", "latest_version_id"),
				),
			},
			// Add a second directory group with an additional file.
			{
				Config: configResourceTwoGroups(nameSuffix, "updated description"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5ads_configuration.test", "configs.#", "2"),
				),
			},
			// Removing the extra group from configs drops those files from the new version
			{
				Config: configResourceWithName(nameSuffix, "updated description", nginxConfContents),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5ads_configuration.test", "configs.#", "1"),
					resource.TestCheckResourceAttr("f5ads_configuration.test", "configs.0.name", "/etc/nginx"),
					resource.TestCheckResourceAttr("f5ads_configuration.test", "configs.0.files.#", "1"),
				),
			},
		},
	})
}

func TestAccConfigResourceWithNamePrefix(t *testing.T) {
	nameSuffix := randomNameSuffix()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: configResourceWithNamePrefix("nginxacc"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5ads_configuration.test", "name_prefix", "nginxacc"),
					resource.TestMatchResourceAttr(
						"f5ads_configuration.test", "name", regexp.MustCompile(`^nginxacc-[0-9a-f]{8}$`),
					),
					resource.TestCheckResourceAttrSet("f5ads_configuration.test", "id"),
					resource.TestCheckResourceAttrSet("f5ads_configuration.test", "latest_version_id"),
				),
			},
			// Switching to an explicit name.
			{
				Config: configResourceWithName(nameSuffix, "replaced", nginxConfContents),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(
						"f5ads_configuration.test", "name", fmt.Sprintf("nginxacc-%d", nameSuffix),
					),
					resource.TestCheckNoResourceAttr("f5ads_configuration.test", "name_prefix"),
				),
			},
			// Switching back to a prefix must generate a new name, not reuse
			// the previous explicit name from state.
			{
				Config: configResourceWithNamePrefix("nginxacc"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestMatchResourceAttr(
						"f5ads_configuration.test", "name", regexp.MustCompile(`^nginxacc-[0-9a-f]{8}$`),
					),
				),
			},
			// Changing the prefix must generate a new name.
			{
				Config: configResourceWithNamePrefix("nginxacc-new"),
				Check: resource.TestMatchResourceAttr(
					"f5ads_configuration.test", "name", regexp.MustCompile(`^nginxacc-new-[0-9a-f]{8}$`),
				),
			},
		},
	})
}

func TestAccConfigResourceValidation(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      configResourceNoName(),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Exactly one of these attributes must be configured: \[name,name_prefix\]`),
			},
			{
				Config:      configResourceBothNames(),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Exactly one of these attributes must be configured: \[name,name_prefix\]`),
			},
			{
				Config:      configResourceInvalidName(),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Attribute name must start with a lowercase letter`),
			},
			{
				Config:      configResourceEmptyConfigs(),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Attribute configs set must contain at least 1 elements`),
			},
		},
	})
}

// tlsNginxConf returns an nginx.conf whose ssl_certificate and
// ssl_certificate_key directives reference the given payload paths.
func tlsNginxConf(certPath, keyPath string) string {
	return fmt.Sprintf(`user nginx;
worker_processes auto;
pid /run/nginx/nginx.pid;

events {
    worker_connections 768;
}

http {
    server {
        listen 443 ssl;
        server_name localhost;

        ssl_certificate     %s;
        ssl_certificate_key %s;

        location / {
            return 200 "Hello from F5 ADS over TLS!";
        }
    }
}
`, certPath, keyPath)
}

// configResourceWithPayloads attaches the managed certificate and key of
// f5ads_certificate.test to the configuration at the given paths.
func configResourceWithPayloads(nameSuffix int, certPath, keyPath string) string {
	return fmt.Sprintf(`
%s

resource "f5ads_configuration" "test" {
  name = "nginxacc-%d"
  configs = [
    {
      name = "/etc/nginx"
      files = [
        {
          name     = "nginx.conf"
          contents = base64encode(%q)
        }
      ]
    }
  ]
  payloads = [
    {
      type      = "managed_certificate"
      object_id = f5ads_certificate.test.id
      paths     = ["%s"]
    },
    {
      type      = "managed_key"
      object_id = f5ads_certificate.test.id
      paths     = ["%s"]
    },
  ]
}
`, certKeyCertificate(
		fmt.Sprintf("nginxacc-cert-%d", nameSuffix),
		fmt.Sprintf("acc-payloads-%d.example.com", nameSuffix),
		false,
	), nameSuffix, tlsNginxConf(certPath, keyPath), certPath, keyPath)
}

// configResourceWithoutPayloads keeps the certificate but detaches it from the configuration.
func configResourceWithoutPayloads(nameSuffix int) string {
	return fmt.Sprintf(`
%s

resource "f5ads_configuration" "test" {
  name = "nginxacc-%d"
  configs = [
    {
      name = "/etc/nginx"
      files = [
        {
          name     = "nginx.conf"
          contents = base64encode(%q)
        }
      ]
    }
  ]
}
`, certKeyCertificate(
		fmt.Sprintf("nginxacc-cert-%d", nameSuffix),
		fmt.Sprintf("acc-payloads-%d.example.com", nameSuffix),
		false,
	), nameSuffix, nginxConfContents)
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
          contents = base64encode(%q)
        }
      ]
    }
  ]
  payloads = [
    %s
  ]
}
`, providerConfig, name, nginxConfContents, payload)
}

// TestAccConfigResourcePayloads requires the f5ads_certificate resource to be
// registered in the provider.
func TestAccConfigResourcePayloads(t *testing.T) {
	nameSuffix := randomNameSuffix()
	const resourceName = "f5ads_configuration.test"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		ExternalProviders:        tlsExternalProvider,
		Steps: []resource.TestStep{
			{
				Config: configResourceWithPayloads(nameSuffix, "/etc/nginx/tls/server.crt", "/etc/nginx/tls/server.key"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttrSet(resourceName, "latest_version_id"),
					resource.TestCheckResourceAttr(resourceName, "payloads.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "payloads.0.type", "managed_certificate"),
					resource.TestCheckResourceAttrPair(resourceName, "payloads.0.object_id", certificateResourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "payloads.0.paths.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "payloads.0.paths.0", "/etc/nginx/tls/server.crt"),
					resource.TestCheckResourceAttr(resourceName, "payloads.1.type", "managed_key"),
					resource.TestCheckResourceAttrPair(resourceName, "payloads.1.object_id", certificateResourceName, "id"),
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
				ExpectError: regexp.MustCompile(`attribute\s+"paths"\s+is\s+required`),
			},
			{
				Config: configResourceInvalidPayload("nginxacc-emptypaths", `{
      type      = "managed_certificate"
      object_id = "cert_123"
      paths     = []
    }`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`list must contain at least 1\s+elements`),
			},
			{
				Config: configResourceInvalidPayload("nginxacc-emptypath", `{
      type      = "managed_certificate"
      object_id = "cert_123"
      paths     = [""]
    }`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`string length must be at\s+least 1`),
			},
		},
	})
}
