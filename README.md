# Terraform provider for F5 Application Delivery Service powered by NGINX

The F5 Application Delivery Service (F5 ADS) Terraform Provider allows you to manage F5 ADS resources via [Terraform](https://www.terraform.io/). This provider enables you to define and manage your F5 ADS infrastructure as code.

> [!IMPORTANT]
> This provider is currently in early development. Features and APIs may change as the provider matures.

## Documentation

- [Terraform Registry Documentation](https://registry.terraform.io/providers/F5Networks/f5ads/latest/docs) _(coming soon)_
- [Examples](./examples/)

## Requirements

- [Terraform](https://www.terraform.io/downloads) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.26 (for development)
- An active F5 ADS organization with valid client credentials.
  - NOTE: You have to subscribe to F5 ADS through your preferred cloud provider in order to manage deployments. See [documentation](https://docs.nginx.com/nginxaas/overview/manage-users-organizations/#access-the-nginxaas-console) for details.

## Installation

Terraform uses the [Terraform Registry](https://registry.terraform.io/) to download and install providers. Add the following to your Terraform configuration:

```hcl
terraform {
  required_providers {
    f5ads = {
      source  = "F5Networks/f5ads"
      version = "0.1.0-alpha"
    }
  }
}

provider "f5ads" {
  geo   = "us"  # Geographic region (e.g., "us", "eu")
}
```

Then run:

```bash
terraform init
```

## Authentication

The provider requires authentication to interact with the F5 ADS API.

You can configure authentication in two ways:

### Provider Configuration

```hcl
provider "f5ads" {
  geo           = "us"
  client_id     = "your-client-id"
  client_secret = "your-client-secret"
}
```

### Environment Variables

```bash
export F5ADS_GEO="us"
export F5ADS_CLIENT_ID="your-client-id"
export F5ADS_CLIENT_SECRET="your-client-secret"
```

## Usage

Here's a basic example of creating an F5 ADS deployment:

```hcl
resource "f5ads_deployment" "example" {
  name                    = "my-deployment"
  capacity                = 20
  waf_enabled             = true
  nginx_config_id         = "cfg_example123"
  nginx_config_version_id = "cv_example456"

  google_cloud_properties = {
    region             = "us-east1"
    network_attachment = "projects/my-project/regions/us-east1/networkAttachments/my-attachment"

    # Exactly one of `managed_public_endpoint` or `private_endpoint` must be set.
    frontend = {
      managed_public_endpoint = {
        acl = [
          {
            source_prefixes = ["0.0.0.0/0"]
            port_range      = "80"
            protocol        = "tcp"
          },
          {
            source_prefixes = ["0.0.0.0/0"]
            port_range      = "443"
            protocol        = "tcp"
          }
        ]
      }
    }
  }
}
```

To expose the deployment through Google Private Service Connect instead, replace the
`managed_public_endpoint` block with a `private_endpoint` block:

```hcl
    frontend = {
      private_endpoint = {
        service_attachment_accept_list = [
          "my-gcp-project",
        ]
      }
    }
```

For more examples, see the [examples](./examples/) directory.

## Available Resources

- `f5ads_deployment` - Manage F5 ADS deployments

## Available Data Sources

- `f5ads_deployment` - Read information about existing deployments

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](./CONTRIBUTING.md) for details on how to get started.

## License

This project is licensed under the [Apache License 2.0](./LICENSE). See the [LICENSE](./LICENSE) file for details.
