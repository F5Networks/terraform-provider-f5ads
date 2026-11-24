package provider

import (
	"fmt"
	"testing"

	testingresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func awsUpstreamNetworkPeerConfig(nameSuffix int, peer string) string {
	config := baseAWSPrivateDeploymentConfig(nameSuffix) + baseAWSCustomerVPCConfig(nameSuffix) + fmt.Sprintf(`
resource "aws_vpc_peering_connection" "first" {
  vpc_id        = aws_vpc.test.id
  peer_vpc_id   = f5ads_deployment.test.aws_cloud_properties.vpc_id
  peer_owner_id = f5ads_deployment.test.aws_cloud_properties.account_id
  tags = {
    Name = "nginxacc-peer-first-%[1]d"
  }
}

resource "aws_vpc" "second" {
  cidr_block = "10.2.0.0/16"
  tags = {
    Name = "nginxacc-vpc-second-%[1]d"
  }
}

resource "aws_vpc_peering_connection" "second" {
  vpc_id        = aws_vpc.second.id
  peer_vpc_id   = f5ads_deployment.test.aws_cloud_properties.vpc_id
  peer_owner_id = f5ads_deployment.test.aws_cloud_properties.account_id
  tags = {
    Name = "nginxacc-peer-second-%[1]d"
  }
}
`, nameSuffix)
	if peer != "" {
		config += fmt.Sprintf(`
resource "f5ads_aws_upstream_network_peer" "test" {
  deployment_id         = f5ads_deployment.test.id
  peering_connection_id = aws_vpc_peering_connection.%s.id
}
`, peer)
	}
	return config
}

func TestAccAWSUpstreamNetworkPeer(t *testing.T) {
	nameSuffix := randomNameSuffix()
	testingresource.ParallelTest(t, testingresource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		ExternalProviders: map[string]testingresource.ExternalProvider{
			"aws": {Source: "hashicorp/aws", VersionConstraint: "~> 6.0"},
		},
		Steps: []testingresource.TestStep{
			{
				Config: awsUpstreamNetworkPeerConfig(nameSuffix, "first"),
				Check: testingresource.ComposeAggregateTestCheckFunc(
					testingresource.TestCheckResourceAttr("aws_vpc.test", "tags.Name", fmt.Sprintf("nginxacc-vpc-%d", nameSuffix)),
					testingresource.TestCheckResourceAttr("aws_vpc.second", "tags.Name", fmt.Sprintf("nginxacc-vpc-second-%d", nameSuffix)),
					testingresource.TestCheckResourceAttr("aws_vpc_peering_connection.first", "tags.Name", fmt.Sprintf("nginxacc-peer-first-%d", nameSuffix)),
					testingresource.TestCheckResourceAttr("aws_vpc_peering_connection.second", "tags.Name", fmt.Sprintf("nginxacc-peer-second-%d", nameSuffix)),
					testingresource.TestCheckResourceAttrPair("f5ads_aws_upstream_network_peer.test", "deployment_id", "f5ads_deployment.test", "id"),
					testingresource.TestCheckResourceAttrPair("f5ads_aws_upstream_network_peer.test", "peering_connection_id", "aws_vpc_peering_connection.first", "id"),
					testingresource.TestCheckResourceAttrSet("f5ads_aws_upstream_network_peer.test", "id"),
					testingresource.TestCheckResourceAttrSet("f5ads_aws_upstream_network_peer.test", "status"),
				),
			},
			{
				ResourceName:      "f5ads_aws_upstream_network_peer.test",
				ImportState:       true,
				ImportStateVerify: true,
				// Connection status can change between refresh and import.
				ImportStateVerifyIgnore: []string{"status"},
			},
			{
				Config: awsUpstreamNetworkPeerConfig(nameSuffix, "second"),
				Check: testingresource.ComposeAggregateTestCheckFunc(
					testingresource.TestCheckResourceAttrPair("f5ads_aws_upstream_network_peer.test", "deployment_id", "f5ads_deployment.test", "id"),
					testingresource.TestCheckResourceAttrPair("f5ads_aws_upstream_network_peer.test", "peering_connection_id", "aws_vpc_peering_connection.second", "id"),
					testingresource.TestCheckResourceAttrSet("f5ads_aws_upstream_network_peer.test", "status"),
				),
			},
			{
				Config: awsUpstreamNetworkPeerConfig(nameSuffix, ""),
				Check: testingresource.TestCheckResourceAttrSet(
					"f5ads_deployment.test", "aws_cloud_properties.vpc_id",
				),
			},
		},
	})
}
