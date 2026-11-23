resource "f5ads_deployment" "pe_aws" {
  name                    = "aws-upstream"
  capacity                = 10
  nginx_config_id         = f5ads_configuration.example.id
  nginx_config_version_id = f5ads_configuration.example.latest_version_id
  aws_cloud_properties = {
    region          = "us-east-1"
    ipv4_cidr_block = "10.10.0.0/24"
    frontend = {
      private_endpoint = {}
    }
  }
}

# F5 ADS accepts peering requests asynchronously; keep auto_accept disabled and
# do not separately manage an AWS peering accepter.
resource "aws_vpc_peering_connection" "upstream_a" {
  vpc_id        = aws_vpc.customer_a.id
  peer_vpc_id   = f5ads_deployment.pe_aws.aws_cloud_properties.vpc_id
  peer_owner_id = f5ads_deployment.pe_aws.aws_cloud_properties.account_id
}

# Each resource manages one peer; multiple peers can be applied concurrently.
resource "f5ads_aws_upstream_network_peer" "upstream_a" {
  deployment_id         = f5ads_deployment.pe_aws.id
  peering_connection_id = aws_vpc_peering_connection.upstream_a.id
}

resource "aws_vpc_peering_connection" "upstream_b" {
  vpc_id        = aws_vpc.customer_b.id
  peer_vpc_id   = f5ads_deployment.pe_aws.aws_cloud_properties.vpc_id
  peer_owner_id = f5ads_deployment.pe_aws.aws_cloud_properties.account_id
}

resource "f5ads_aws_upstream_network_peer" "upstream_b" {
  deployment_id         = f5ads_deployment.pe_aws.id
  peering_connection_id = aws_vpc_peering_connection.upstream_b.id
}
