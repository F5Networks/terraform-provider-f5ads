package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"testing"

	configs "github.com/F5Networks/terraform-provider-f5ads/internal/provider/clients/configs/2026-07-31"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
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

resource "terraform_data" "configs" {
  input = [
    {
      name = "/etc/nginx"
      files = [
        {
          name     = "nginx.conf"
          contents = base64encode(%[4]q)
        }
      ]
    }
  ]
}

resource "f5ads_configuration" "test" {
  name        = "nginxacc-%[2]d"
  description = "%[3]s"
  configs     = terraform_data.configs.output
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
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectUnknownValue("f5ads_configuration.test", tfjsonpath.New("configs")),
					},
				},
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

resource "terraform_data" "payload_paths" {
  input = {
    certificate = ["%[4]s"]
    key         = ["%[5]s"]
  }
}

resource "f5ads_configuration" "test" {
  name = "nginxacc-%[2]d"
  configs = [
    {
      name = "/etc/nginx"
      files = [
        {
          name     = "nginx.conf"
          contents = base64encode(%[3]q)
        }
      ]
    }
  ]
  payloads = [
    {
      type      = "managed_certificate"
      object_id = f5ads_certificate.test.id
      paths     = terraform_data.payload_paths.output.certificate
    },
    {
      type      = "managed_key"
      object_id = f5ads_certificate.test.id
      paths     = terraform_data.payload_paths.output.key
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
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectUnknownValue(resourceName, tfjsonpath.New("payloads").AtSliceIndex(0).AtMapKey("paths")),
						plancheck.ExpectUnknownValue(resourceName, tfjsonpath.New("payloads").AtSliceIndex(1).AtMapKey("paths")),
					},
				},
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

const indexHTMLContents = `<html><body>Hello from aux!</body></html>
`

func testListValue(t *testing.T, elementType attr.Type, values interface{}) types.List {
	t.Helper()
	value, diags := types.ListValueFrom(context.Background(), elementType, values)
	if diags.HasError() {
		t.Fatalf("ListValueFrom: %v", diags)
	}
	return value
}

func testSetValue(t *testing.T, elementType attr.Type, values interface{}) types.Set {
	t.Helper()
	value, diags := types.SetValueFrom(context.Background(), elementType, values)
	if diags.HasError() {
		t.Fatalf("SetValueFrom: %v", diags)
	}
	return value
}

// TestConfigReplaceRequestAlwaysSendsAux verifies that the PUT body always
// contains an explicit aux field, and that it is an empty array when no aux
// groups are configured. The API keeps previously known aux content when the
// field is omitted.
func TestConfigReplaceRequestAlwaysSendsAux(t *testing.T) {
	nginxConf := testSetValue(t, configGroupType, []ConfigGroupModel{{
		Name: types.StringValue("/etc/nginx"),
		Files: testSetValue(t, configFileType, []ConfigFileModel{{
			Name:     types.StringValue("nginx.conf"),
			Contents: types.StringValue(base64.StdEncoding.EncodeToString([]byte(nginxConfContents))),
		}}),
	}})

	tests := []struct {
		name    string
		aux     types.List
		wantLen int
	}{
		{name: "null aux (aux blocks removed)", aux: types.ListNull(auxGroupType), wantLen: 0},
		{name: "empty aux", aux: types.ListValueMust(auxGroupType, nil), wantLen: 0},
		{
			name: "one aux group",
			aux: testListValue(t, auxGroupType, []AuxGroupModel{{
				Name: types.StringValue("/srv/www"),
				Files: testListValue(t, configFileType, []ConfigFileModel{
					{
						Name:     types.StringValue("index.html"),
						Contents: types.StringValue(base64.StdEncoding.EncodeToString([]byte(indexHTMLContents))),
					},
					{Name: types.StringValue("empty.txt"), Contents: types.StringNull()},
				}),
			}}),
			wantLen: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := &configResourceModel{Configs: nginxConf, Aux: test.aux}
			cfg, err := configRequestFromModel(context.Background(), model)
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
		Configs: types.SetNull(configGroupType),
		Aux: testListValue(t, auxGroupType, []AuxGroupModel{{
			Name:  types.StringValue("/srv/www"),
			Files: testListValue(t, configFileType, []ConfigFileModel{{Name: types.StringValue("index.html"), Contents: types.StringValue("not base64!")}}),
		}}),
	}
	if _, err := configRequestFromModel(context.Background(), model); err == nil {
		t.Fatal("configRequestFromModel() expected error for invalid base64 aux contents")
	}
}

// configResourceWithAux returns a configuration with nginx.conf and one aux
// directory containing a static HTML file.
func configResourceWithAux(nameSuffix int) string {
	return fmt.Sprintf(`
%s

resource "terraform_data" "config_files" {
  input = [{
    name     = "nginx.conf"
    contents = base64encode(%[3]q)
  }]
}

resource "terraform_data" "aux" {
  input = [{
    name = "/srv/www"
    files = [{
      name     = "index.html"
      contents = base64encode(%[4]q)
    }]
  }]
}

resource "f5ads_configuration" "test" {
  name = "nginxacc-%[2]d"
  configs = [
    {
      name = "/etc/nginx"
      files = terraform_data.config_files.output
    }
  ]
  aux = terraform_data.aux.output
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
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectUnknownValue("f5ads_configuration.test", tfjsonpath.New("configs").AtSliceIndex(0).AtMapKey("files")),
						plancheck.ExpectUnknownValue("f5ads_configuration.test", tfjsonpath.New("aux")),
					},
				},
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
