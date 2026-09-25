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

events {
    worker_connections 768;
}

http {
    server {
        listen 80;
        server_name localhost;

        location / {
            return 200 "Hello from F5 ADS NGINX!";
        }
    }
}
`

const updatedNginxConfContents = `user nginx;
worker_processes auto;
pid /run/nginx/nginx.pid;

events {
    worker_connections 768;
}

http {
    server {
        listen 80;
        server_name localhost;

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

func b64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
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
          contents = "%s"
        }
      ]
    }
  ]
}
`, providerConfig, nameSuffix, description, b64(conf))
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
          contents = "%s"
        }
      ]
    },
    {
      name = "/etc/nginx/conf.d"
      files = [
        {
          name     = "site.conf"
          contents = "%s"
        }
      ]
    }
  ]
}
`, providerConfig, nameSuffix, description, b64(nginxConfContents), b64(siteConfContents))
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
          contents = "%s"
        }
      ]
    }
  ]
}
`, providerConfig, prefix, b64(nginxConfContents))
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
          contents = "%s"
        }
      ]
    }
  ]
}
`, providerConfig, b64(nginxConfContents))
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
          contents = "%s"
        }
      ]
    }
  ]
}
`, providerConfig, b64(nginxConfContents))
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
          contents = "%s"
        }
      ]
    }
  ]
}
`, providerConfig, b64(nginxConfContents))
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

func TestAccConfigResource(t *testing.T) {
	nameSuffix := randomNameSuffix()
	name := fmt.Sprintf("nginxacc-%d", nameSuffix)

	resource.Test(t, resource.TestCase{
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
						"f5ads_configuration.test", "configs.0.files.0.contents", b64(nginxConfContents),
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
						"f5ads_configuration.test", "configs.0.files.0.contents", b64(updatedNginxConfContents),
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

func TestAccConfigResourceNamePrefix(t *testing.T) {
	nameSuffix := randomNameSuffix()

	resource.Test(t, resource.TestCase{
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
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      configResourceNoName(),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Exactly one of name or name_prefix must be configured`),
			},
			{
				Config:      configResourceBothNames(),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Exactly one of name or name_prefix must be configured`),
			},
			{
				Config:      configResourceInvalidName(),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`File name is invalid`),
			},
			{
				Config:      configResourceEmptyConfigs(),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Config must contain at least 1 content`),
			},
		},
	})
}
