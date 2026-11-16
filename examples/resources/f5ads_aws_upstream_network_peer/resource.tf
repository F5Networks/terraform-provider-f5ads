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

# Create peering requests from your application VPCs to the deployment VPC.
# F5 ADS handles acceptance asynchronously; do not enable auto_accept
# or separately manage an AWS peering accepter for these connections.
resource "aws_vpc_peering_connection" "upstream_a" {
  vpc_id        = aws_vpc.customer_a.id
  peer_vpc_id   = f5ads_deployment.pe_aws.aws_cloud_properties.vpc_id
  peer_owner_id = f5ads_deployment.pe_aws.aws_cloud_properties.account_id
}

resource "aws_vpc_peering_connection" "upstream_b" {
  vpc_id        = aws_vpc.customer_b.id
  peer_vpc_id   = f5ads_deployment.pe_aws.aws_cloud_properties.vpc_id
  peer_owner_id = f5ads_deployment.pe_aws.aws_cloud_properties.account_id
}

# Each instance owns one ID. Both can apply concurrently to this deployment.
resource "f5ads_aws_upstream_network_peer" "upstream_a" {
  deployment_id         = f5ads_deployment.pe_aws.id
  peering_connection_id = aws_vpc_peering_connection.upstream_a.id
}

resource "f5ads_aws_upstream_network_peer" "upstream_b" {
  deployment_id         = f5ads_deployment.pe_aws.id
  peering_connection_id = aws_vpc_peering_connection.upstream_b.id
}
