terraform {
  required_providers {
    f5ads = {
      source = "registry.terraform.io/F5Networks/f5ads"
    }
  }
}


provider "f5ads" {
  geo           = "us"
  client_id     = var.f5ads_client_id
  client_secret = var.f5ads_client_secret
}

output "config_id" {
  value = f5ads_configuration.example.id
}

output "version_id" {
  value = f5ads_configuration.example.latest_version_id
}

output "organization_id" {
  value = f5ads_configuration.example.organization_id
}
