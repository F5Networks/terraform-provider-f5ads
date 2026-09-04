//go:build tools

package tools

import (
	// tfplugindocs generates Terraform provider documentation from schema definitions.
	// Run 'make generate' to regenerate the docs/ directory.
	_ "github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs"
)
