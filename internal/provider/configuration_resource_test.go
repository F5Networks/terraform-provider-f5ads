package provider

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"testing"

	configs "github.com/F5Networks/terraform-provider-f5ads/internal/provider/clients/configs/2026-07-31"
	"github.com/hashicorp/terraform-plugin-framework/types"
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

const indexHTMLContents = `<html><body>Hello from aux!</body></html>
`

// TestConfigReplaceRequestAlwaysSendsAux verifies that the PUT body always
// contains an explicit aux field, and that it is an empty array when no aux
// groups are configured. The API keeps previously known aux content when the
// field is omitted.
func TestConfigReplaceRequestAlwaysSendsAux(t *testing.T) {
	nginxConf := []ConfigGroupModel{{
		Name: types.StringValue("/etc/nginx"),
		Files: []ConfigFileModel{{
			Name:     types.StringValue("nginx.conf"),
			Contents: types.StringValue(base64.StdEncoding.EncodeToString([]byte(nginxConfContents))),
		}},
	}}

	tests := []struct {
		name    string
		aux     []ConfigGroupModel
		wantLen int
	}{
		{name: "nil aux (aux blocks removed)", aux: nil, wantLen: 0},
		{name: "empty aux", aux: []ConfigGroupModel{}, wantLen: 0},
		{
			name: "one aux group",
			aux: []ConfigGroupModel{{
				Name: types.StringValue("/srv/www"),
				Files: []ConfigFileModel{
					{
						Name:     types.StringValue("index.html"),
						Contents: types.StringValue(base64.StdEncoding.EncodeToString([]byte(indexHTMLContents))),
					},
					{Name: types.StringValue("empty.txt"), Contents: types.StringNull()},
				},
			}},
			wantLen: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := &configResourceModel{Configs: nginxConf, Aux: test.aux}
			cfg, err := configRequestFromModel(model)
			if err != nil {
				t.Fatalf("configRequestFromModel() error = %v", err)
			}

			body, err := json.Marshal(configs.NginxConfigReplaceRequest{Config: cfg})
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}

			var decoded struct {
				Config map[string]json.RawMessage `json:"config"`
			}
			if err := json.Unmarshal(body, &decoded); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}

			raw, ok := decoded.Config["aux"]
			if !ok {
				t.Fatalf("PUT body omits \"aux\"; want it sent explicitly. body: %s", body)
			}
			var aux []json.RawMessage
			if err := json.Unmarshal(raw, &aux); err != nil || aux == nil {
				t.Fatalf("\"aux\" = %s, want a JSON array", raw)
			}
			if len(aux) != test.wantLen {
				t.Errorf("len(aux) = %d, want %d. body: %s", len(aux), test.wantLen, body)
			}
			if test.wantLen == 0 && string(raw) != "[]" {
				t.Errorf("\"aux\" = %s, want []", raw)
			}
		})
	}
}

func TestConfigRequestInvalidAuxBase64(t *testing.T) {
	model := &configResourceModel{
		Aux: []ConfigGroupModel{{
			Name:  types.StringValue("/srv/www"),
			Files: []ConfigFileModel{{Name: types.StringValue("index.html"), Contents: types.StringValue("not base64!")}},
		}},
	}
	if _, err := configRequestFromModel(model); err == nil {
		t.Fatal("configRequestFromModel() expected error for invalid base64 aux contents")
	}
}

// configResourceWithAux returns a configuration with nginx.conf and one aux
// directory containing a static HTML file.
func configResourceWithAux(nameSuffix int) string {
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
  aux = [
    {
      name = "/srv/www"
      files = [
        {
          name     = "index.html"
          contents = base64encode(%q)
        }
      ]
    }
  ]
}
`, providerConfig, nameSuffix, nginxConfContents, indexHTMLContents)
}

// configResourceWithoutAux is the same configuration with every aux block removed.
func configResourceWithoutAux(nameSuffix int) string {
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
`, providerConfig, nameSuffix, nginxConfContents)
}

func TestAccConfigResourceAux(t *testing.T) {
	nameSuffix := randomNameSuffix()

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: configResourceWithAux(nameSuffix),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5ads_configuration.test", "aux.#", "1"),
					resource.TestCheckResourceAttr("f5ads_configuration.test", "aux.0.name", "/srv/www"),
					resource.TestCheckResourceAttr("f5ads_configuration.test", "aux.0.files.#", "1"),
					resource.TestCheckResourceAttr("f5ads_configuration.test", "aux.0.files.0.name", "index.html"),
					resource.TestCheckResourceAttr(
						"f5ads_configuration.test", "aux.0.files.0.contents",
						base64.StdEncoding.EncodeToString([]byte(indexHTMLContents)),
					),
				),
			},
			{
				ResourceName:      "f5ads_configuration.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Removing every aux block sends aux: [] in the PUT. Aux in state is
			// read back from the new latest version, so it must be empty here.
			{
				Config: configResourceWithoutAux(nameSuffix),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("f5ads_configuration.test", "aux.#"),
					resource.TestCheckResourceAttr("f5ads_configuration.test", "configs.#", "1"),
				),
			},
			{
				RefreshState: true,
				Check:        resource.TestCheckNoResourceAttr("f5ads_configuration.test", "aux.#"),
			},
		},
	})
}
