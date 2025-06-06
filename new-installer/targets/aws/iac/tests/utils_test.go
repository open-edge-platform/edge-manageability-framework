// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package aws_iac_test

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"

	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	DefaultRegion = "us-west-2"
)

type AWSS3BackendConfig struct {
	Region string `json:"region"`
	Bucket string `json:"bucket"`
	Key    string `json:"key"`
}

func GetSubnetByID(vpcID string, subnetID string, region string) (*ec2.Subnet, error) {
	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	svc := ec2.New(sess)
	input := &ec2.DescribeSubnetsInput{
		SubnetIds: []*string{aws.String(subnetID)},
	}

	result, err := svc.DescribeSubnets(input)
	if err != nil {
		return nil, err
	}

	if len(result.Subnets) == 0 {
		return nil, fmt.Errorf("no subnet found")
	}

	for _, subnet := range result.Subnets {
		if *subnet.VpcId == vpcID {
			return subnet, nil
		}
	}

	return nil, fmt.Errorf("no subnet found")
}

func GetInternetGatewaysByTags(region string, tags map[string][]string) ([]*ec2.InternetGateway, error) {
	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	svc := ec2.New(sess)
	filter := make([]*ec2.Filter, 0, len(tags))
	for key, values := range tags {
		filter = append(filter, &ec2.Filter{
			Name:   aws.String("tag:" + key),
			Values: aws.StringSlice(values),
		})
	}
	input := &ec2.DescribeInternetGatewaysInput{
		Filters: filter,
	}

	result, err := svc.DescribeInternetGateways(input)
	if err != nil {
		return nil, err
	}

	if len(result.InternetGateways) == 0 {
		return nil, fmt.Errorf("no internet gateway found with the specified tags")
	}
	return result.InternetGateways, nil
}

func GetNATGatewaysByTags(region string, tags map[string][]string) ([]*ec2.NatGateway, error) {
	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	svc := ec2.New(sess)
	filter := make([]*ec2.Filter, 0, len(tags))
	for key, values := range tags {
		filter = append(filter, &ec2.Filter{
			Name:   aws.String("tag:" + key),
			Values: aws.StringSlice(values),
		})
	}
	input := &ec2.DescribeNatGatewaysInput{
		Filter: filter,
	}

	result, err := svc.DescribeNatGateways(input)
	if err != nil {
		return nil, err
	}
	if len(result.NatGateways) == 0 {
		return nil, fmt.Errorf("no NAT gateways found with the specified tags")
	}
	return result.NatGateways, nil
}

// This function creates a VPC and three public and private subnets in the specified AWS region.
func CreateVPC(region string, name string) (string, []string, []string, error) {
	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		return "", []string{}, []string{}, fmt.Errorf("failed to create session: %w", err)
	}
	ec2Client := ec2.New(sess)

	// Create VPC
	createVpcInput := &ec2.CreateVpcInput{
		CidrBlock: aws.String("10.250.0.0/16"),
		TagSpecifications: []*ec2.TagSpecification{
			{
				ResourceType: aws.String(ec2.ResourceTypeVpc),
				Tags: []*ec2.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(name),
					},
					{
						Key:   aws.String("CustomerTag"),
						Value: aws.String("test-customer"),
					},
				},
			},
		},
	}
	vpcOutput, err := ec2Client.CreateVpc(createVpcInput)
	if err != nil {
		return "", nil, []string{}, fmt.Errorf("failed to create VPC: %w", err)
	}
	vpcID := *vpcOutput.Vpc.VpcId
	log.Printf("Created VPC with ID: %s", vpcID)

	// Create Internet Gateway
	createInternetGatewayInput := &ec2.CreateInternetGatewayInput{
		TagSpecifications: []*ec2.TagSpecification{
			{
				ResourceType: aws.String(ec2.ResourceTypeInternetGateway),
				Tags: []*ec2.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(fmt.Sprintf("%s-igw", name)),
					},
					{
						Key:   aws.String("CustomerTag"),
						Value: aws.String("test-customer"),
					},
				},
			},
		},
	}
	igwOutput, err := ec2Client.CreateInternetGateway(createInternetGatewayInput)
	if err != nil {
		return "", nil, []string{}, fmt.Errorf("failed to create Internet Gateway: %w", err)
	}
	igwID := *igwOutput.InternetGateway.InternetGatewayId
	log.Printf("Created Internet Gateway with ID: %s", igwID)

	// Attach Internet Gateway to VPC
	attachIgwInput := &ec2.AttachInternetGatewayInput{
		InternetGatewayId: aws.String(igwID),
		VpcId:             aws.String(vpcID),
	}
	_, err = ec2Client.AttachInternetGateway(attachIgwInput)
	if err != nil {
		return "", nil, []string{}, fmt.Errorf("failed to attach Internet Gateway to VPC: %w", err)
	}
	log.Printf("Attached Internet Gateway %s to VPC %s", igwID, vpcID)

	// Create Subnets
	publicSubnetIDs := []string{}
	availabilityZones := []string{"a", "b", "c"}
	for i, zone := range availabilityZones {
		cidrBlock := fmt.Sprintf("10.250.%d.0/24", i)
		createSubnetInput := &ec2.CreateSubnetInput{
			CidrBlock:        aws.String(cidrBlock),
			VpcId:            aws.String(vpcID),
			AvailabilityZone: aws.String(region + zone),
			TagSpecifications: []*ec2.TagSpecification{
				{
					ResourceType: aws.String(ec2.ResourceTypeSubnet),
					Tags: []*ec2.Tag{
						{
							Key:   aws.String("Name"),
							Value: aws.String(fmt.Sprintf("%s-subnet-%s", name, zone)),
						},
						{
							Key:   aws.String("CustomerTag"),
							Value: aws.String("test-customer"),
						},
					},
				},
			},
		}
		subnetOutput, err := ec2Client.CreateSubnet(createSubnetInput)
		if err != nil {
			return "", nil, []string{}, fmt.Errorf("failed to create public subnet in zone %s: %w", zone, err)
		}
		subnetID := *subnetOutput.Subnet.SubnetId
		log.Printf("Created Public Subnet with ID: %s in zone %s", subnetID, zone)
		publicSubnetIDs = append(publicSubnetIDs, subnetID)
	}

	// Create Route Table
	createRouteTableInput := &ec2.CreateRouteTableInput{
		VpcId: aws.String(vpcID),
		TagSpecifications: []*ec2.TagSpecification{
			{
				ResourceType: aws.String(ec2.ResourceTypeRouteTable),
				Tags: []*ec2.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(fmt.Sprintf("%s-rtb", name)),
					},
					{
						Key:   aws.String("CustomerTag"),
						Value: aws.String("test-customer"),
					},
				},
			},
		},
	}
	routeTableOutput, err := ec2Client.CreateRouteTable(createRouteTableInput)
	if err != nil {
		return "", nil, []string{}, fmt.Errorf("failed to create route table: %w", err)
	}
	routeTableID := *routeTableOutput.RouteTable.RouteTableId
	log.Printf("Created Route Table with ID: %s", routeTableID)

	// Create Route to Internet Gateway
	createRouteInput := &ec2.CreateRouteInput{
		RouteTableId:         aws.String(routeTableID),
		DestinationCidrBlock: aws.String("0.0.0.0/0"),
		GatewayId:            aws.String(igwID),
	}
	_, err = ec2Client.CreateRoute(createRouteInput)
	if err != nil {
		return "", nil, []string{}, fmt.Errorf("failed to create route to Internet Gateway: %w", err)
	}
	log.Printf("Created route to Internet Gateway %s in Route Table %s", igwID, routeTableID)

	// Associate Subnets with Route Table
	for _, subnetID := range publicSubnetIDs {
		associateRouteTableInput := &ec2.AssociateRouteTableInput{
			RouteTableId: aws.String(routeTableID),
			SubnetId:     aws.String(subnetID),
		}
		_, err := ec2Client.AssociateRouteTable(associateRouteTableInput)
		if err != nil {
			return "", nil, []string{}, fmt.Errorf("failed to associate subnet %s with route table: %w", subnetID, err)
		}
		log.Printf("Associated Subnet %s with Route Table %s", subnetID, routeTableID)
	}

	privateSubnetIDs := []string{}
	for i, zone := range availabilityZones {
		cidrBlock := fmt.Sprintf("10.250.%d.0/24", i+3) // Offset CIDR blocks for private subnets
		createPrivateSubnetInput := &ec2.CreateSubnetInput{
			CidrBlock:        aws.String(cidrBlock),
			VpcId:            aws.String(vpcID),
			AvailabilityZone: aws.String(region + zone),
			TagSpecifications: []*ec2.TagSpecification{
				{
					ResourceType: aws.String(ec2.ResourceTypeSubnet),
					Tags: []*ec2.Tag{
						{
							Key:   aws.String("Name"),
							Value: aws.String(fmt.Sprintf("%s-private-subnet-%s", name, zone)),
						},
						{
							Key:   aws.String("CustomerTag"),
							Value: aws.String("test-customer"),
						},
					},
				},
			},
		}
		privateSubnetOutput, err := ec2Client.CreateSubnet(createPrivateSubnetInput)
		if err != nil {
			return "", nil, []string{}, fmt.Errorf("failed to create private subnet in zone %s: %w", zone, err)
		}
		privateSubnetID := *privateSubnetOutput.Subnet.SubnetId
		log.Printf("Created Private Subnet with ID: %s in zone %s", privateSubnetID, zone)
		privateSubnetIDs = append(privateSubnetIDs, privateSubnetID)
	}

	// Create NAT Gateways and associate them with private subnets
	for i, zone := range availabilityZones {
		// Create Elastic IP for NAT Gateway
		allocateAddressInput := &ec2.AllocateAddressInput{
			Domain: aws.String(ec2.DomainTypeVpc),
		}
		eipOutput, err := ec2Client.AllocateAddress(allocateAddressInput)
		if err != nil {
			return "", nil, []string{}, fmt.Errorf("failed to allocate Elastic IP for NAT Gateway in zone %s: %w", zone, err)
		}
		eipAllocationID := *eipOutput.AllocationId
		log.Printf("Allocated Elastic IP with Allocation ID: %s for NAT Gateway in zone %s", eipAllocationID, zone)

		// Create NAT Gateway
		createNatGatewayInput := &ec2.CreateNatGatewayInput{
			SubnetId:     aws.String(privateSubnetIDs[i]),
			AllocationId: aws.String(eipAllocationID),
			TagSpecifications: []*ec2.TagSpecification{
				{
					ResourceType: aws.String(ec2.ResourceTypeNatgateway),
					Tags: []*ec2.Tag{
						{
							Key:   aws.String("Name"),
							Value: aws.String(fmt.Sprintf("%s-natgw-%s", name, zone)),
						},
						{
							Key:   aws.String("CustomerTag"),
							Value: aws.String("test-customer"),
						},
						{
							Key:   aws.String("VPC"),
							Value: aws.String(vpcID),
						},
					},
				},
			},
		}
		natGatewayOutput, err := ec2Client.CreateNatGateway(createNatGatewayInput)
		if err != nil {
			return "", nil, []string{}, fmt.Errorf("failed to create NAT Gateway in zone %s: %w", zone, err)
		}
		natGatewayID := *natGatewayOutput.NatGateway.NatGatewayId
		log.Printf("Created NAT Gateway with ID: %s in zone %s", natGatewayID, zone)

		// Wait for NAT Gateway to become available
		describeNatGatewaysInput := &ec2.DescribeNatGatewaysInput{
			NatGatewayIds: []*string{aws.String(natGatewayID)},
		}
		for {
			time.Sleep(10 * time.Second)
			describeNatGatewaysOutput, err := ec2Client.DescribeNatGateways(describeNatGatewaysInput)
			if err != nil {
				return "", nil, []string{}, fmt.Errorf("failed to describe NAT Gateway %s: %w", natGatewayID, err)
			}
			if len(describeNatGatewaysOutput.NatGateways) > 0 && *describeNatGatewaysOutput.NatGateways[0].State == ec2.NatGatewayStateAvailable {
				log.Printf("NAT Gateway %s is now available", natGatewayID)
				break
			}
		}

		// Create a route table for the private subnet
		createPrivateRouteTableInput := &ec2.CreateRouteTableInput{
			VpcId: aws.String(vpcID),
			TagSpecifications: []*ec2.TagSpecification{
				{
					ResourceType: aws.String(ec2.ResourceTypeRouteTable),
					Tags: []*ec2.Tag{
						{
							Key:   aws.String("Name"),
							Value: aws.String(fmt.Sprintf("%s-private-rtb-%s", name, zone)),
						},
						{
							Key:   aws.String("CustomerTag"),
							Value: aws.String("test-customer"),
						},
					},
				},
			},
		}
		privateRouteTableOutput, err := ec2Client.CreateRouteTable(createPrivateRouteTableInput)
		if err != nil {
			return "", nil, []string{}, fmt.Errorf("failed to create private route table in zone %s: %w", zone, err)
		}
		privateRouteTableID := *privateRouteTableOutput.RouteTable.RouteTableId
		log.Printf("Created Private Route Table with ID: %s in zone %s", privateRouteTableID, zone)

		// Create a default route to the NAT Gateway
		createPrivateRouteInput := &ec2.CreateRouteInput{
			RouteTableId:         aws.String(privateRouteTableID),
			DestinationCidrBlock: aws.String("0.0.0.0/0"),
			NatGatewayId:         aws.String(natGatewayID),
		}
		_, err = ec2Client.CreateRoute(createPrivateRouteInput)
		if err != nil {
			return "", nil, []string{}, fmt.Errorf("failed to create default route to NAT Gateway %s in private route table %s: %w", natGatewayID, privateRouteTableID, err)
		}
		log.Printf("Created default route to NAT Gateway %s in Private Route Table %s", natGatewayID, privateRouteTableID)

		// Associate the private subnet with the private route table
		associatePrivateRouteTableInput := &ec2.AssociateRouteTableInput{
			RouteTableId: aws.String(privateRouteTableID),
			SubnetId:     aws.String(privateSubnetIDs[i]),
		}
		_, err = ec2Client.AssociateRouteTable(associatePrivateRouteTableInput)
		if err != nil {
			return "", nil, []string{}, fmt.Errorf("failed to associate private subnet %s with private route table %s: %w", privateSubnetIDs[i], privateRouteTableID, err)
		}
		log.Printf("Associated Private Subnet %s with Private Route Table %s", privateSubnetIDs[i], privateRouteTableID)
	}

	return vpcID, publicSubnetIDs, privateSubnetIDs, nil
}

func DeleteVPC(region string, vpcID string) error {
	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	ec2Client := ec2.New(sess)

	// Retrieve and delete all NAT gateways associated with the VPC
	describeNatGatewaysInput := &ec2.DescribeNatGatewaysInput{
		Filter: []*ec2.Filter{
			{
				Name:   aws.String("tag:VPC"),
				Values: []*string{&vpcID},
			},
		},
	}
	natGatewaysOutput, err := ec2Client.DescribeNatGateways(describeNatGatewaysInput)
	if err != nil {
		return fmt.Errorf("failed to describe NAT gateways: %w", err)
	}

	for _, natGateway := range natGatewaysOutput.NatGateways {
		// Release the Elastic IP associated with the NAT Gateway
		if natGateway.NatGatewayAddresses != nil {
			for _, address := range natGateway.NatGatewayAddresses {
				if address.AllocationId != nil {
					releaseAddressInput := &ec2.ReleaseAddressInput{
						AllocationId: address.AllocationId,
					}
					_, err := ec2Client.ReleaseAddress(releaseAddressInput)
					if err != nil {
						return fmt.Errorf("failed to release Elastic IP %s associated with NAT gateway %s: %w", *address.AllocationId, *natGateway.NatGatewayId, err)
					}
					log.Printf("Released Elastic IP with Allocation ID: %s associated with NAT Gateway ID: %s", *address.AllocationId, *natGateway.NatGatewayId)
				}
			}
		}

		// Delete the NAT Gateway
		deleteNatGatewayInput := &ec2.DeleteNatGatewayInput{
			NatGatewayId: natGateway.NatGatewayId,
		}
		_, err := ec2Client.DeleteNatGateway(deleteNatGatewayInput)
		if err != nil {
			return fmt.Errorf("failed to delete NAT gateway %s: %w", *natGateway.NatGatewayId, err)
		}
		log.Printf("Deleted NAT Gateway with ID: %s", *natGateway.NatGatewayId)
	}

	// Retrieve and delete all internet gateways associated with the VPC
	describeInternetGatewaysInput := &ec2.DescribeInternetGatewaysInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("attachment.vpc-id"),
				Values: []*string{&vpcID},
			},
		},
	}
	internetGatewaysOutput, err := ec2Client.DescribeInternetGateways(describeInternetGatewaysInput)
	if err != nil {
		return fmt.Errorf("failed to describe internet gateways: %w", err)
	}

	for _, igw := range internetGatewaysOutput.InternetGateways {
		detachIgwInput := &ec2.DetachInternetGatewayInput{
			InternetGatewayId: igw.InternetGatewayId,
			VpcId:             aws.String(vpcID),
		}
		_, err := ec2Client.DetachInternetGateway(detachIgwInput)
		if err != nil {
			return fmt.Errorf("failed to detach internet gateway %s: %w", *igw.InternetGatewayId, err)
		}
		log.Printf("Detached Internet Gateway with ID: %s", *igw.InternetGatewayId)

		deleteIgwInput := &ec2.DeleteInternetGatewayInput{
			InternetGatewayId: igw.InternetGatewayId,
		}
		_, err = ec2Client.DeleteInternetGateway(deleteIgwInput)
		if err != nil {
			return fmt.Errorf("failed to delete internet gateway %s: %w", *igw.InternetGatewayId, err)
		}
		log.Printf("Deleted Internet Gateway with ID: %s", *igw.InternetGatewayId)
	}

	// Retrieve and delete all subnets associated with the VPC
	describeSubnetsInput := &ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []*string{&vpcID},
			},
		},
	}
	subnetsOutput, err := ec2Client.DescribeSubnets(describeSubnetsInput)
	if err != nil {
		return fmt.Errorf("failed to describe subnets: %w", err)
	}

	for _, subnet := range subnetsOutput.Subnets {
		deleteSubnetInput := &ec2.DeleteSubnetInput{
			SubnetId: subnet.SubnetId,
		}
		_, err := ec2Client.DeleteSubnet(deleteSubnetInput)
		if err != nil {
			return fmt.Errorf("failed to delete subnet %s: %w", *subnet.SubnetId, err)
		}
		log.Printf("Deleted Subnet with ID: %s", *subnet.SubnetId)
	}
	// Retrieve and delete all route tables associated with the VPC
	describeRouteTablesInput := &ec2.DescribeRouteTablesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []*string{&vpcID},
			},
		},
	}
	routeTablesOutput, err := ec2Client.DescribeRouteTables(describeRouteTablesInput)
	if err != nil {
		return fmt.Errorf("failed to describe route tables: %w", err)
	}

	for _, routeTable := range routeTablesOutput.RouteTables {
		// Skip the main route table
		if routeTable.Associations != nil {
			for _, assoc := range routeTable.Associations {
				if assoc.Main != nil && *assoc.Main {
					continue
				}
			}
		}

		// Delete default route (0.0.0.0/0) from the route table
		if routeTable.Routes != nil {
			for _, route := range routeTable.Routes {
				if route.DestinationCidrBlock != nil && *route.DestinationCidrBlock == "0.0.0.0/0" {
					deleteRouteInput := &ec2.DeleteRouteInput{
						RouteTableId:         routeTable.RouteTableId,
						DestinationCidrBlock: route.DestinationCidrBlock,
					}
					_, err := ec2Client.DeleteRoute(deleteRouteInput)
					if err != nil {
						return fmt.Errorf("failed to delete default route from route table %s: %w", *routeTable.RouteTableId, err)
					}
					log.Printf("Deleted default route (0.0.0.0/0) from Route Table with ID: %s", *routeTable.RouteTableId)
				}
			}
		}

		deleteRouteTableInput := &ec2.DeleteRouteTableInput{
			RouteTableId: routeTable.RouteTableId,
		}
		_, err := ec2Client.DeleteRouteTable(deleteRouteTableInput)
		if err != nil {
			// Might be an error if the route table is default route table
			continue
		}
		log.Printf("Deleted Route Table with ID: %s", *routeTable.RouteTableId)
	}

	// Retrieve and delete all security groups associated with the VPC
	describeSecurityGroupsInput := &ec2.DescribeSecurityGroupsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []*string{&vpcID},
			},
		},
	}
	securityGroupsOutput, err := ec2Client.DescribeSecurityGroups(describeSecurityGroupsInput)
	if err != nil {
		return fmt.Errorf("failed to describe security groups: %w", err)
	}

	for _, securityGroup := range securityGroupsOutput.SecurityGroups {
		// Skip the default security group
		if *securityGroup.GroupName == "default" {
			continue
		}

		deleteSecurityGroupInput := &ec2.DeleteSecurityGroupInput{
			GroupId: securityGroup.GroupId,
		}
		_, err := ec2Client.DeleteSecurityGroup(deleteSecurityGroupInput)
		if err != nil {
			return fmt.Errorf("failed to delete security group %s: %w", *securityGroup.GroupId, err)
		}
		log.Printf("Deleted Security Group with ID: %s", *securityGroup.GroupId)
	}
	deleteVpcInput := &ec2.DeleteVpcInput{
		VpcId: aws.String(vpcID),
	}
	_, err = ec2Client.DeleteVpc(deleteVpcInput)
	if err != nil {
		return fmt.Errorf("failed to delete VPC: %w", err)
	}
	log.Printf("Deleted VPC with ID: %s", vpcID)
	return nil
}

func CreateJumpHost(vpcID string, subnetID string, region string, ipCIDRAllowlist []string) (string, string, string, error) {
	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create session: %w", err)
	}
	ec2Client := ec2.New(sess)

	// Create a security group for the jump host
	createSecurityGroupInput := &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(fmt.Sprintf("jump-host-sg-%s", strings.ToLower(rand.Text()[:8]))),
		Description: aws.String("Security group for jump host"),
		VpcId:       aws.String(vpcID),
		TagSpecifications: []*ec2.TagSpecification{
			{
				ResourceType: aws.String(ec2.ResourceTypeSecurityGroup),
				Tags: []*ec2.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(fmt.Sprintf("jump-host-sg-%s", strings.ToLower(rand.Text()[:8]))),
					},
					{
						Key:   aws.String("CustomerTag"),
						Value: aws.String("test-customer"),
					},
				},
			},
		},
	}
	securityGroupOutput, err := ec2Client.CreateSecurityGroup(createSecurityGroupInput)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create security group: %w", err)
	}
	securityGroupID := *securityGroupOutput.GroupId
	log.Printf("Created Security Group with ID: %s", securityGroupID)

	// Get the public IP address of the machine running the test
	var publicIP string
	if socksProxy := os.Getenv("SOCKS_PROXY"); socksProxy != "" {
		// Use SOCKS_PROXY to get the public IP address
		log.Printf("Using SOCKS_PROXY: %s\n", socksProxy)
		cmd := exec.Command("curl", "--socks5", socksProxy, "https://checkip.amazonaws.com")
		output, err := cmd.Output()
		if err != nil {
			return "", "", "", fmt.Errorf("failed to get public IP address using SOCKS_PROXY: %w", err)
		}
		publicIP = strings.TrimSpace(string(output))
	} else {
		// Directly get the public IP address
		resp, err := http.Get("https://checkip.amazonaws.com")
		if err != nil {
			return "", "", "", fmt.Errorf("failed to get public IP address: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", "", "", fmt.Errorf("failed to read response body: %w", err)
		}
		publicIP = strings.TrimSpace(string(body))
	}

	myCIDR := fmt.Sprintf("%s/32", publicIP) // Allow only this machine's IP
	log.Printf("Public IP of the machine running the test: %s\n", myCIDR)

	ipAllowRanges := []*ec2.IpRange{{CidrIp: aws.String(myCIDR)}}
	for _, ip := range ipCIDRAllowlist {
		if ip != "" {
			ipAllowRanges = append(ipAllowRanges, &ec2.IpRange{CidrIp: aws.String(ip)})
		}
	}

	authorizeIngressInput := &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId: aws.String(securityGroupID),
		IpPermissions: []*ec2.IpPermission{
			{
				IpProtocol: aws.String("-1"), // -1 allows all protocols
				IpRanges:   ipAllowRanges,    // Allow traffic from this IP
			},
		},
	}
	_, err = ec2Client.AuthorizeSecurityGroupIngress(authorizeIngressInput)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to authorize security group ingress: %w", err)
	}
	log.Printf("Authorized inbound SSH traffic for Security Group ID: %s", securityGroupID)

	// Generate a temporary SSH key pair
	privateKey, publicKey, err := generateSSHKeyPair()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to generate SSH key pair: %w", err)
	}

	// Generate a random name for the key pair
	keyName := fmt.Sprintf("jump-host-key-%s", strings.ToLower(rand.Text()[:8]))

	// Import the public key to AWS
	importKeyPairInput := &ec2.ImportKeyPairInput{
		KeyName:           aws.String(keyName),
		PublicKeyMaterial: []byte(publicKey),
	}
	_, err = ec2Client.ImportKeyPair(importKeyPairInput)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to import SSH key pair: %w", err)
	}
	log.Printf("Imported SSH key pair for jump host with name: %s", keyName)

	// Create the jump host instance
	createInstanceInput := &ec2.RunInstancesInput{
		ImageId:      aws.String("ami-0a605bc2ef5707a18"), // Ubuntu 24.04 LTS in us-west-2
		InstanceType: aws.String(ec2.InstanceTypeT3Micro),
		MinCount:     aws.Int64(1),
		MaxCount:     aws.Int64(1),
		KeyName:      aws.String(keyName),
		NetworkInterfaces: []*ec2.InstanceNetworkInterfaceSpecification{
			{
				AssociatePublicIpAddress: aws.Bool(true),
				DeviceIndex:              aws.Int64(0),
				SubnetId:                 aws.String(subnetID),
				Groups:                   []*string{&securityGroupID},
			},
		},
		TagSpecifications: []*ec2.TagSpecification{
			{
				ResourceType: aws.String(ec2.ResourceTypeInstance),
				Tags: []*ec2.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(fmt.Sprintf("jump-host-%s", strings.ToLower(rand.Text()[:8]))),
					},
					{
						Key:   aws.String("CustomerTag"),
						Value: aws.String("test-customer"),
					},
				},
			},
		},
	}
	runInstancesOutput, err := ec2Client.RunInstances(createInstanceInput)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create jump host instance: %w", err)
	}
	instanceID := *runInstancesOutput.Instances[0].InstanceId
	log.Printf("Created Jump Host Instance with ID: %s", instanceID)

	// Wait for the instance to be in the running state and retrieve its public IP address
	describeInstancesInput := &ec2.DescribeInstancesInput{
		InstanceIds: []*string{&instanceID},
	}

	var jumphostIP string
	for range 5 {
		log.Printf("Waiting for Jump Host Instance to be in running state...")
		time.Sleep(10 * time.Second)
		describeInstancesOutput, err := ec2Client.DescribeInstances(describeInstancesInput)
		if err != nil {
			log.Printf("Failed to describe instance: %v, continue...\n", err)
			continue
		}

		instance := describeInstancesOutput.Reservations[0].Instances[0]
		if *instance.State.Name == ec2.InstanceStateNameRunning && instance.PublicIpAddress != nil {
			jumphostIP = *instance.PublicIpAddress
			log.Printf("Jump Host Instance is running with Public IP: %s", jumphostIP)
			break
		}
	}

	if jumphostIP == "" {
		return instanceID, privateKey, jumphostIP, fmt.Errorf("failed to retrieve public IP address of the jump host instance")
	}

	// Write the private key to a temporary file
	privateKeyFile, err := os.CreateTemp("", "jumphost-key-*.pem")
	defer os.Remove(privateKeyFile.Name()) // Clean up the temporary file after use

	if err != nil {
		return instanceID, privateKey, jumphostIP, fmt.Errorf("failed to create temporary private key file: %w", err)
	}
	defer privateKeyFile.Close()

	if _, err := privateKeyFile.WriteString(privateKey); err != nil {
		return instanceID, privateKey, jumphostIP, fmt.Errorf("failed to write private key to temporary file: %w", err)
	}

	// Set the file permissions to read-only for the owner
	if err := os.Chmod(privateKeyFile.Name(), 0400); err != nil {
		return instanceID, privateKey, jumphostIP, fmt.Errorf("failed to set permissions on temporary private key file: %w", err)
	}
	log.Printf("Private key written to temporary file: %s", privateKeyFile.Name())

	// Wait for the instance to be reachable via SSH
	var reachable bool = false
	for range 10 {
		time.Sleep(10 * time.Second)
		var sshCmd *exec.Cmd
		if socksProxy := os.Getenv("SOCKS_PROXY"); socksProxy != "" {
			sshCmd = exec.Command("ssh", "-T", "-o", fmt.Sprintf("ProxyCommand=nc -x %s %%h %%p", socksProxy), "-o", "StrictHostKeyChecking=no", "-i", privateKeyFile.Name(), fmt.Sprintf("ubuntu@%s", jumphostIP), "true")
		} else {
			sshCmd = exec.Command("ssh", "-T", "-o", "StrictHostKeyChecking=no", "-i", privateKeyFile.Name(), fmt.Sprintf("ubuntu@%s", jumphostIP), "true")
		}
		if err := sshCmd.Run(); err == nil {
			reachable = true
			break
		}

	}
	if !reachable {
		return instanceID, privateKey, jumphostIP, fmt.Errorf("jump host instance is not reachable via SSH at %s", jumphostIP)
	}
	log.Printf("Successfully connected to Jump Host Instance via SSH at %s", jumphostIP)

	return instanceID, privateKey, jumphostIP, nil
}

func generateSSHKeyPair() (string, string, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %w", err)
	}

	privateKeyPEM := &bytes.Buffer{}
	if err := pem.Encode(privateKeyPEM, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}); err != nil {
		return "", "", fmt.Errorf("failed to encode private key: %w", err)
	}

	publicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate public key: %w", err)
	}

	return privateKeyPEM.String(), string(ssh.MarshalAuthorizedKey(publicKey)), nil
}

func DeleteJumpHost(instanceID string, region string) error {
	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	ec2Client := ec2.New(sess)

	// Terminate the jump host instance
	terminateInstancesInput := &ec2.TerminateInstancesInput{
		InstanceIds: []*string{&instanceID},
	}
	_, err = ec2Client.TerminateInstances(terminateInstancesInput)
	if err != nil {
		return fmt.Errorf("failed to terminate jump host instance: %w", err)
	}

	// Wait for the instance to be terminated
	describeInstancesInput := &ec2.DescribeInstancesInput{
		InstanceIds: []*string{&instanceID},
	}

	for range 20 {
		log.Printf("Waiting for Jump Host Instance to be terminated...")
		time.Sleep(10 * time.Second)
		describeInstancesOutput, err := ec2Client.DescribeInstances(describeInstancesInput)
		if err != nil {
			log.Printf("Failed to describe instance: %v, continue...\n", err)
			continue
		}

		instance := describeInstancesOutput.Reservations[0].Instances[0]
		if *instance.State.Name == ec2.InstanceStateNameTerminated {
			log.Printf("Jump Host Instance with ID %s has been terminated", instanceID)
			break
		}
	}

	log.Printf("Terminated Jump Host Instance with ID: %s", instanceID)

	return nil
}

func StartSshuttle(jumphostIP string, jumphostKey string, remoteCIDRBlock string) error {
	// Create a temporary file for the private key
	privateKeyFile, err := os.CreateTemp("", "jumphost-key-*.pem")
	defer os.Remove(privateKeyFile.Name()) // Clean up the temporary file after sshuttle started
	if err != nil {
		return fmt.Errorf("failed to create temporary private key file: %w", err)
	}
	if _, err := privateKeyFile.WriteString(jumphostKey); err != nil {
		return fmt.Errorf("failed to write private key to temporary file: %w", err)
	}
	if err := privateKeyFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary private key file: %w", err)
	}
	// Set the file permissions to read/write for the owner only
	if err := os.Chmod(privateKeyFile.Name(), 0400); err != nil {
		return fmt.Errorf("failed to set permissions on temporary private key file: %w", err)
	}
	// Construct the sshuttle command
	var cmd *exec.Cmd
	if socksProxy := os.Getenv("SOCKS_PROXY"); socksProxy != "" {
		cmd = exec.Command("sshuttle", "--pidfile", "/tmp/sshuttle.pid", "-D", "-e", fmt.Sprintf("ssh -o ProxyCommand='nc -x %s %%h %%p' -i %s -o StrictHostKeyChecking=no", socksProxy, privateKeyFile.Name()), "-r", fmt.Sprintf("ubuntu@%s", jumphostIP), remoteCIDRBlock)
	} else {
		cmd = exec.Command("sshuttle", "--pidfile", "/tmp/sshuttle.pid", "-D", "-r", fmt.Sprintf("ubuntu@%s", jumphostIP), "--ssh-cmd", fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=no", privateKeyFile.Name()), remoteCIDRBlock)
	}
	log.Printf("Starting sshuttle command: %s", cmd.String())

	// It will actually run in the background due to the -D flag
	// Use /tmp/sshuttle.pid to track the process
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start sshuttle command: %w", err)
	}
	time.Sleep(5 * time.Second) // Wait for sshuttle to establish the connection
	// Print the PID of the sshuttle process
	pid, err := os.ReadFile("/tmp/sshuttle.pid")
	if err != nil {
		log.Printf("Failed to read sshuttle PID file: %v", err)
	} else {
		log.Printf("sshuttle is running with PID: %s", strings.TrimSpace(string(pid)))
	}
	return nil
}

func StopSshuttle() error {
	// Read the PID from the sshuttle PID file
	pidFile := "/tmp/sshuttle.pid"
	pidData, err := os.ReadFile(pidFile)
	if err != nil {
		// Failed to read it, assume the process is not running
		return nil
	}

	// Convert the PID to an integer
	pid := strings.TrimSpace(string(pidData))
	cmd := exec.Command("kill", pid)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to terminate sshuttle process with PID %s: %w", pid, err)
	}

	log.Println("Stopped sshuttle successfully")
	return nil
}
