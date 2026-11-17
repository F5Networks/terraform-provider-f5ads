package provider

import (
	"fmt"
	"testing"

	testingresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
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
resource "aws_subnet" "test" {
  vpc_id     = aws_vpc.test.id
  cidr_block = "10.1.1.0/24"
  tags = {
    Name = "nginxacc-subnet-%[1]d"
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

resource "f5ads_aws_private_endpoint_connection" "test" {
  deployment_id = f5ads_deployment.test.id
  endpoint_id   = aws_vpc_endpoint.test.id
}
`, nameSuffix)
	}
	return config
}

func TestAccAWSPrivateEndpointConnection(t *testing.T) {
	nameSuffix := randomNameSuffix()
	testingresource.ParallelTest(t, testingresource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		ExternalProviders: map[string]testingresource.ExternalProvider{
			"aws": {Source: "hashicorp/aws", VersionConstraint: "~> 6.0"},
		},
		Steps: []testingresource.TestStep{
			{
				Config: awsPrivateEndpointConnectionConfig(nameSuffix, "account"),
				Check: testingresource.ComposeAggregateTestCheckFunc(
					testingresource.TestCheckResourceAttrSet("f5ads_deployment.test", "private_link_service_endpoint_name"),
					testingresource.TestCheckResourceAttrPair("f5ads_aws_private_endpoint_connection.test", "deployment_id", "f5ads_deployment.test", "id"),
					testingresource.TestCheckResourceAttrPair("f5ads_aws_private_endpoint_connection.test", "endpoint_id", "data.aws_caller_identity.current", "account_id"),
					testingresource.TestCheckResourceAttrSet("f5ads_aws_private_endpoint_connection.test", "id"),
					testingresource.TestCheckNoResourceAttr("f5ads_aws_private_endpoint_connection.test", "status"),
				),
			},
			{
				ResourceName:      "f5ads_aws_private_endpoint_connection.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: awsPrivateEndpointConnectionConfig(nameSuffix, ""),
				Check: testingresource.TestCheckResourceAttrSet(
					"f5ads_deployment.test", "private_link_service_endpoint_name",
				),
			},
			{
				Config: awsPrivateEndpointConnectionConfig(nameSuffix, "endpoint"),
				Check: testingresource.ComposeAggregateTestCheckFunc(
					testingresource.TestCheckResourceAttrPair("aws_subnet.test", "vpc_id", "aws_vpc.test", "id"),
					testingresource.TestCheckResourceAttr("aws_vpc.test", "tags.Name", fmt.Sprintf("nginxacc-vpc-%d", nameSuffix)),
					testingresource.TestCheckResourceAttr("aws_subnet.test", "tags.Name", fmt.Sprintf("nginxacc-subnet-%d", nameSuffix)),
					testingresource.TestCheckResourceAttr("aws_vpc_endpoint.test", "tags.Name", fmt.Sprintf("nginxacc-endpoint-%d", nameSuffix)),
					testingresource.TestCheckResourceAttrPair("aws_vpc_endpoint.test", "vpc_id", "aws_vpc.test", "id"),
					testingresource.TestCheckTypeSetElemAttrPair("aws_vpc_endpoint.test", "subnet_ids.*", "aws_subnet.test", "id"),
					testingresource.TestCheckResourceAttrPair("aws_vpc_endpoint.test", "service_name", "f5ads_deployment.test", "private_link_service_endpoint_name"),
					testingresource.TestCheckResourceAttr("aws_vpc_endpoint.test", "vpc_endpoint_type", "Interface"),
					testingresource.TestCheckResourceAttrPair("f5ads_aws_private_endpoint_connection.test", "deployment_id", "f5ads_deployment.test", "id"),
					testingresource.TestCheckResourceAttrPair("f5ads_aws_private_endpoint_connection.test", "endpoint_id", "aws_vpc_endpoint.test", "id"),
					testingresource.TestCheckResourceAttrSet("f5ads_aws_private_endpoint_connection.test", "id"),
					testingresource.TestCheckResourceAttrSet("f5ads_aws_private_endpoint_connection.test", "status"),
				),
			},
			{
				ResourceName:            "f5ads_aws_private_endpoint_connection.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"status"},
			},
			{
				Config: awsPrivateEndpointConnectionConfig(nameSuffix, ""),
				Check: testingresource.TestCheckResourceAttrSet(
					"f5ads_deployment.test", "private_link_service_endpoint_name",
				),
			},
		},
	})
}
