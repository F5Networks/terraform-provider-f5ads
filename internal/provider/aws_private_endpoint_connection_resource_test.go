package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func awsPrivateEndpointConnectionConfig(nameSuffix int, connectionType string) string {
	config := baseAWSPrivateDeploymentConfig(nameSuffix)
	switch connectionType {
	case "account":
		config += `
resource "f5ads_aws_private_endpoint_connection" "test" {
  deployment_id = f5ads_deployment.test.id
  endpoint_id   = data.aws_caller_identity.current.account_id
}
`
	case "endpoint":
		config += baseAWSCustomerVPCConfig(nameSuffix) + fmt.Sprintf(`
resource "aws_vpc" "second" {
  cidr_block = "10.2.0.0/16"
  tags = {
    Name = "nginxacc-vpc-second-%[1]d"
  }
}

resource "aws_subnet" "test" {
  vpc_id     = aws_vpc.test.id
  cidr_block = "10.1.1.0/24"
  tags = {
    Name = "nginxacc-subnet-%[1]d"
  }
}

resource "aws_subnet" "second" {
  vpc_id     = aws_vpc.second.id
  cidr_block = "10.2.1.0/24"
  tags = {
    Name = "nginxacc-subnet-second-%[1]d"
  }
}

resource "aws_vpc_endpoint" "test" {
  vpc_id            = aws_vpc.test.id
  subnet_ids        = [aws_subnet.test.id]
  service_name      = f5ads_deployment.test.private_link_service_endpoint_name
  vpc_endpoint_type = "Interface"
  tags = {
    Name = "nginxacc-endpoint-%[1]d"
  }
}

resource "aws_vpc_endpoint" "second" {
  vpc_id            = aws_vpc.second.id
  subnet_ids        = [aws_subnet.second.id]
  service_name      = f5ads_deployment.test.private_link_service_endpoint_name
  vpc_endpoint_type = "Interface"
  tags = {
    Name = "nginxacc-endpoint-second-%[1]d"
  }
}

resource "f5ads_aws_private_endpoint_connection" "test" {
  deployment_id = f5ads_deployment.test.id
  endpoint_id   = aws_vpc_endpoint.test.id
}

resource "f5ads_aws_private_endpoint_connection" "second" {
  deployment_id = f5ads_deployment.test.id
  endpoint_id   = aws_vpc_endpoint.second.id
}
`, nameSuffix)
	}
	return config
}

func TestAccAWSPrivateEndpointConnection(t *testing.T) {
	nameSuffix := randomNameSuffix()
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		ExternalProviders: map[string]resource.ExternalProvider{
			"aws": {Source: "hashicorp/aws", VersionConstraint: "~> 6.0"},
		},
		Steps: []resource.TestStep{
			{
				Config: awsPrivateEndpointConnectionConfig(nameSuffix, "account"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("f5ads_deployment.test", "private_link_service_endpoint_name"),
					resource.TestCheckResourceAttrPair("f5ads_aws_private_endpoint_connection.test", "deployment_id", "f5ads_deployment.test", "id"),
					resource.TestCheckResourceAttrPair("f5ads_aws_private_endpoint_connection.test", "endpoint_id", "data.aws_caller_identity.current", "account_id"),
					resource.TestCheckResourceAttrSet("f5ads_aws_private_endpoint_connection.test", "id"),
				),
			},
			{
				ResourceName:      "f5ads_aws_private_endpoint_connection.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: awsPrivateEndpointConnectionConfig(nameSuffix, ""),
				Check: resource.TestCheckResourceAttrSet(
					"f5ads_deployment.test", "private_link_service_endpoint_name",
				),
			},
			{
				Config: awsPrivateEndpointConnectionConfig(nameSuffix, "endpoint"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("aws_subnet.test", "vpc_id", "aws_vpc.test", "id"),
					resource.TestCheckResourceAttr("aws_vpc.test", "tags.Name", fmt.Sprintf("nginxacc-vpc-%d", nameSuffix)),
					resource.TestCheckResourceAttr("aws_subnet.test", "tags.Name", fmt.Sprintf("nginxacc-subnet-%d", nameSuffix)),
					resource.TestCheckResourceAttr("aws_vpc_endpoint.test", "tags.Name", fmt.Sprintf("nginxacc-endpoint-%d", nameSuffix)),
					resource.TestCheckResourceAttrPair("aws_vpc_endpoint.test", "vpc_id", "aws_vpc.test", "id"),
					resource.TestCheckTypeSetElemAttrPair("aws_vpc_endpoint.test", "subnet_ids.*", "aws_subnet.test", "id"),
					resource.TestCheckResourceAttrPair("aws_vpc_endpoint.test", "service_name", "f5ads_deployment.test", "private_link_service_endpoint_name"),
					resource.TestCheckResourceAttr("aws_vpc_endpoint.test", "vpc_endpoint_type", "Interface"),
					resource.TestCheckResourceAttrPair("aws_subnet.second", "vpc_id", "aws_vpc.second", "id"),
					resource.TestCheckResourceAttr("aws_vpc.second", "tags.Name", fmt.Sprintf("nginxacc-vpc-second-%d", nameSuffix)),
					resource.TestCheckResourceAttr("aws_subnet.second", "tags.Name", fmt.Sprintf("nginxacc-subnet-second-%d", nameSuffix)),
					resource.TestCheckResourceAttr("aws_vpc_endpoint.second", "tags.Name", fmt.Sprintf("nginxacc-endpoint-second-%d", nameSuffix)),
					resource.TestCheckResourceAttrPair("aws_vpc_endpoint.second", "vpc_id", "aws_vpc.second", "id"),
					resource.TestCheckTypeSetElemAttrPair("aws_vpc_endpoint.second", "subnet_ids.*", "aws_subnet.second", "id"),
					resource.TestCheckResourceAttrPair("aws_vpc_endpoint.second", "service_name", "f5ads_deployment.test", "private_link_service_endpoint_name"),
					resource.TestCheckResourceAttr("aws_vpc_endpoint.second", "vpc_endpoint_type", "Interface"),
					resource.TestCheckResourceAttrPair("f5ads_aws_private_endpoint_connection.test", "deployment_id", "f5ads_deployment.test", "id"),
					resource.TestCheckResourceAttrPair("f5ads_aws_private_endpoint_connection.test", "endpoint_id", "aws_vpc_endpoint.test", "id"),
					resource.TestCheckResourceAttrSet("f5ads_aws_private_endpoint_connection.test", "id"),
					resource.TestCheckResourceAttrPair("f5ads_aws_private_endpoint_connection.second", "deployment_id", "f5ads_deployment.test", "id"),
					resource.TestCheckResourceAttrPair("f5ads_aws_private_endpoint_connection.second", "endpoint_id", "aws_vpc_endpoint.second", "id"),
					resource.TestCheckResourceAttrSet("f5ads_aws_private_endpoint_connection.second", "id"),
				),
			},
			{
				ResourceName:      "f5ads_aws_private_endpoint_connection.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:      "f5ads_aws_private_endpoint_connection.second",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: awsPrivateEndpointConnectionConfig(nameSuffix, ""),
				Check: resource.TestCheckResourceAttrSet(
					"f5ads_deployment.test", "private_link_service_endpoint_name",
				),
			},
		},
	})
}
