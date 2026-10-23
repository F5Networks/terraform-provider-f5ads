# Terraform provider for F5 Application Delivery Service powered by NGINX

The F5 Application Delivery Service (F5 ADS) Terraform Provider allows you to manage F5 ADS resources via [Terraform](https://www.terraform.io/). This provider enables you to define and manage your F5 ADS infrastructure as code.

## Documentation

- [Terraform Registry Documentation](https://registry.terraform.io/providers/F5Networks/f5ads/latest/docs)

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
      version = "0.1.0"
    }
  }
}

provider "f5ads" {
  geo   = "us"  # Geographic region: "us", "eu", "apac", or "ca"
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

Refer to the following examples:

- [Deployments](https://registry.terraform.io/providers/F5Networks/f5ads/latest/docs/resources/deployment#example-usage)
- [Configurations](https://registry.terraform.io/providers/F5Networks/f5ads/latest/docs/resources/configuration#example-usage)
- [Certificates](https://registry.terraform.io/providers/F5Networks/f5ads/latest/docs/resources/certificate#example-usage)

## Available Resources

- `f5ads_deployment` - Manage F5 ADS deployments
- `f5ads_configuration` - Manage F5 ADS configurations
- `f5ads_certificate` - Manage F5 ADS certificates

## Available Data Sources

- `f5ads_deployment` - Read information about existing deployments

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](./CONTRIBUTING.md) for details on how to get started.

## License

This project is licensed under the [Apache License 2.0](./LICENSE). See the [LICENSE](./LICENSE) file for details.
