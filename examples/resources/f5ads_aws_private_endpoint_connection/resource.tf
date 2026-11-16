# Create the deployment first to obtain its AWS PrivateLink service name.
resource "f5ads_deployment" "pe_aws" {
  name                    = "private-ingress"
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

# Terraform then creates the endpoint in your VPC using that service name.
resource "aws_vpc_endpoint" "to_f5ads_a" {
  vpc_id            = aws_vpc.main.id
  service_name      = f5ads_deployment.pe_aws.private_link_service_endpoint_name
  vpc_endpoint_type = "Interface"
  subnet_ids        = [aws_subnet.a.id]
}

# Finally, allow this endpoint without waiting for connection acceptance.
# Alternatively, set endpoint_id to a quoted 12-digit AWS account ID.
# A deployment's allowlist cannot mix account IDs and VPC endpoint IDs.
resource "f5ads_aws_private_endpoint_connection" "endpoint_a" {
  deployment_id = f5ads_deployment.pe_aws.id
  endpoint_id   = aws_vpc_endpoint.to_f5ads_a.id
}

# Additional connections can target the same deployment concurrently.
resource "aws_vpc_endpoint" "to_f5ads_b" {
  vpc_id            = aws_vpc.main.id
  service_name      = f5ads_deployment.pe_aws.private_link_service_endpoint_name
  vpc_endpoint_type = "Interface"
  subnet_ids        = [aws_subnet.b.id]
}

resource "f5ads_aws_private_endpoint_connection" "endpoint_b" {
  deployment_id = f5ads_deployment.pe_aws.id
  endpoint_id   = aws_vpc_endpoint.to_f5ads_b.id
}
