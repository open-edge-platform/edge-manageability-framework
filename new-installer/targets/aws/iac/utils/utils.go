// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package aws_iac

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"golang.org/x/crypto/ssh"
)

const (
	DefaultRegion        = "us-west-2"
	DefaultCustomerTag   = "test-customer"
	DefaultJumphostAMIID = "ami-0a605bc2ef5707a18" // Ubuntu 24.04 LTS in us-west-2
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
						Value: aws.String(DefaultCustomerTag),
					},
				},
			},
		},
	}
	vpcOutput, err := ec2Client.CreateVpc(createVpcInput)
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to create VPC: %w", err)
	}
	vpcID := *vpcOutput.Vpc.VpcId
	log.Printf("Created VPC with ID: %s", vpcID)

	// Create Internet Gateway
	igwID, err := createInternetGateway(name, ec2Client, vpcID)
	if err != nil {
		return "", nil, nil, err
	}

	availabilityZones := []string{"a", "b", "c"}
	// Create Public Subnets
	publicSubnetIDs, err := createSubnets(
		availabilityZones,
		vpcID,
		region,
		name,
		ec2Client,
		[]string{
			"10.250.0.0/24",
			"10.250.1.0/24",
			"10.250.2.0/24",
		})
	if err != nil {
		return "", nil, nil, err
	}

	// Create Route Table
	routeTableID, err := createRouteTable(vpcID, name, ec2Client)
	if err != nil {
		return "", nil, nil, err
	}

	// Create Route to Internet Gateway
	createRouteInput := &ec2.CreateRouteInput{
		RouteTableId:         aws.String(routeTableID),
		DestinationCidrBlock: aws.String("0.0.0.0/0"),
		GatewayId:            aws.String(igwID),
	}
	_, err = ec2Client.CreateRoute(createRouteInput)
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to create route to Internet Gateway: %w", err)
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
			return "", nil, nil, fmt.Errorf("failed to associate subnet %s with route table: %w", subnetID, err)
		}
		log.Printf("Associated Subnet %s with Route Table %s", subnetID, routeTableID)
	}

	// Create private subnets
	privateSubnetIDs, err := createSubnets(
		availabilityZones,
		vpcID,
		region,
		name,
		ec2Client,
		[]string{
			"10.250.3.0/24",
			"10.250.4.0/24",
			"10.250.5.0/24",
		})
	if err != nil {
		return "", nil, nil, err
	}

	// Create NAT Gateways and associate them with private subnets
	natGatewayIDs, err := createNATGateways(availabilityZones, ec2Client, privateSubnetIDs, name, vpcID)
	if err != nil {
		return "", nil, nil, err
	}

	for _, natGatewayID := range natGatewayIDs {
		// Wait for NAT Gateway to become available
		describeNatGatewaysInput := &ec2.DescribeNatGatewaysInput{
			NatGatewayIds: []*string{aws.String(natGatewayID)},
		}
		for {
			time.Sleep(10 * time.Second)
			describeNatGatewaysOutput, err := ec2Client.DescribeNatGateways(describeNatGatewaysInput)
			if err != nil {
				return "", nil, nil, fmt.Errorf("failed to describe NAT Gateway %s: %w", natGatewayID, err)
			}
			if len(describeNatGatewaysOutput.NatGateways) > 0 && *describeNatGatewaysOutput.NatGateways[0].State == ec2.NatGatewayStateAvailable {
				log.Printf("NAT Gateway %s is now available", natGatewayID)
				break
			}
		}
	}
	privateRouteTableIDs := make([]string, 0)
	for _, subnetID := range privateSubnetIDs {
		// Create Private Subnet Route Tables
		routeTableID, err := createRouteTable(vpcID, name+"-"+subnetID, ec2Client)
		if err != nil {
			return "", nil, nil, err
		}
		privateRouteTableIDs = append(privateRouteTableIDs, routeTableID)
	}

	for i, privateRouteTableID := range privateRouteTableIDs {
		// Create a default route to the NAT Gateway
		createPrivateRouteInput := &ec2.CreateRouteInput{
			RouteTableId:         aws.String(privateRouteTableID),
			DestinationCidrBlock: aws.String("0.0.0.0/0"),
			NatGatewayId:         aws.String(natGatewayIDs[i]),
		}
		_, err = ec2Client.CreateRoute(createPrivateRouteInput)
		if err != nil {
			return "", nil, nil, fmt.Errorf("failed to create default route to NAT Gateway %s in private route table %s: %w", natGatewayIDs[i], privateRouteTableID, err)
		}
		log.Printf("Created default route to NAT Gateway %s in Private Route Table %s", natGatewayIDs[i], privateRouteTableID)

		// Associate the private subnet with the private route table
		associatePrivateRouteTableInput := &ec2.AssociateRouteTableInput{
			RouteTableId: aws.String(privateRouteTableID),
			SubnetId:     aws.String(privateSubnetIDs[i]),
		}
		_, err = ec2Client.AssociateRouteTable(associatePrivateRouteTableInput)
		if err != nil {
			return "", nil, nil, fmt.Errorf("failed to associate private subnet %s with private route table %s: %w", privateSubnetIDs[i], privateRouteTableID, err)
		}
		log.Printf("Associated Private Subnet %s with Private Route Table %s", privateSubnetIDs[i], privateRouteTableID)
	}

	return vpcID, publicSubnetIDs, privateSubnetIDs, nil
}

func createNATGateways(availabilityZones []string, ec2Client *ec2.EC2, privateSubnetIDs []string, name string, vpcID string) ([]string, error) {
	natGatewayIDs := make([]string, 0)
	for i, zone := range availabilityZones {
		// Create Elastic IP for NAT Gateway
		allocateAddressInput := &ec2.AllocateAddressInput{
			Domain: aws.String(ec2.DomainTypeVpc),
		}
		eipOutput, err := ec2Client.AllocateAddress(allocateAddressInput)
		if err != nil {
			return nil, fmt.Errorf("failed to allocate Elastic IP for NAT Gateway in zone %s: %w", zone, err)
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
							Value: aws.String(DefaultCustomerTag),
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
			return nil, fmt.Errorf("failed to create NAT Gateway in zone %s: %w", zone, err)
		}
		natGatewayID := *natGatewayOutput.NatGateway.NatGatewayId
		natGatewayIDs = append(natGatewayIDs, natGatewayID)
		log.Printf("Created NAT Gateway with ID: %s in zone %s", natGatewayID, zone)
	}
	return natGatewayIDs, nil
}

func createRouteTable(vpcID string, name string, ec2Client *ec2.EC2) (string, error) {
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
						Value: aws.String(DefaultCustomerTag),
					},
				},
			},
		},
	}
	routeTableOutput, err := ec2Client.CreateRouteTable(createRouteTableInput)
	if err != nil {
		return "", fmt.Errorf("failed to create route table: %w", err)
	}
	routeTableID := *routeTableOutput.RouteTable.RouteTableId
	log.Printf("Created Route Table with ID: %s", routeTableID)
	return routeTableID, nil
}

func createSubnets(availabilityZones []string, vpcID string, region string, name string, ec2Client *ec2.EC2, subnetCIDRs []string) ([]string, error) {
	publicSubnetIDs := []string{}
	for i, zone := range availabilityZones {
		createSubnetInput := &ec2.CreateSubnetInput{
			CidrBlock:        aws.String(subnetCIDRs[i]),
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
							Value: aws.String(DefaultCustomerTag),
						},
					},
				},
			},
		}
		subnetOutput, err := ec2Client.CreateSubnet(createSubnetInput)
		if err != nil {
			return nil, fmt.Errorf("failed to create public subnet in zone %s: %w", zone, err)
		}
		subnetID := *subnetOutput.Subnet.SubnetId
		log.Printf("Created Public Subnet with ID: %s in zone %s", subnetID, zone)
		publicSubnetIDs = append(publicSubnetIDs, subnetID)
	}
	return publicSubnetIDs, nil
}

func createInternetGateway(name string, ec2Client *ec2.EC2, vpcID string) (string, error) {
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
						Value: aws.String(DefaultCustomerTag),
					},
				},
			},
		},
	}
	igwOutput, err := ec2Client.CreateInternetGateway(createInternetGatewayInput)
	if err != nil {
		return "", fmt.Errorf("failed to create Internet Gateway: %w", err)
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
		return "", fmt.Errorf("failed to attach Internet Gateway to VPC: %w", err)
	}
	log.Printf("Attached Internet Gateway %s to VPC %s", igwID, vpcID)
	return igwID, nil
}

func DeleteVPC(region string, vpcID string) error {
	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	ec2Client := ec2.New(sess)
	if err := deleteNATGateways(vpcID, ec2Client); err != nil {
		return err
	}
	if err := deleteInternetGateway(vpcID, ec2Client); err != nil {
		return err
	}
	if err := deleteSubnets(vpcID, ec2Client); err != nil {
		return err
	}
	if err := deleteRouteTables(vpcID, ec2Client); err != nil {
		return err
	}
	if err := deleteSecurityGroups(vpcID, ec2Client); err != nil {
		return err
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

func deleteSecurityGroups(vpcID string, ec2Client *ec2.EC2) error {
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
	return nil
}

func deleteRouteTables(vpcID string, ec2Client *ec2.EC2) error {
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
	return nil
}

func deleteSubnets(vpcID string, ec2Client *ec2.EC2) error {
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
	return nil
}

func deleteInternetGateway(vpcID string, ec2Client *ec2.EC2) error {
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
	return nil
}

func deleteNATGateways(vpcID string, ec2Client *ec2.EC2) error {
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
	return nil
}

func CreateJumpHost(vpcID string, subnetID string, region string, ipCIDRAllowlist []string) (string, string, string, error) {
	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create session: %w", err)
	}
	ec2Client := ec2.New(sess)
	publicIP, err := getMyPublicIP()
	if err != nil {
		return "", "", "", err
	}
	myCIDR := fmt.Sprintf("%s/32", publicIP) // Allow only this machine's IP
	log.Printf("Public IP of the machine running the test: %s\n", myCIDR)
	ipAllowRanges := []*ec2.IpRange{{CidrIp: aws.String(myCIDR)}}
	for _, ip := range ipCIDRAllowlist {
		if ip != "" {
			ipAllowRanges = append(ipAllowRanges, &ec2.IpRange{CidrIp: aws.String(ip)})
		}
	}
	securityGroupID, err := createJumphostSecurityGroup(vpcID, ec2Client, ipAllowRanges)
	if err != nil {
		return "", "", "", err
	}
	privateKey, acmKeyName, err := createJumphostSSHKeyPair(ec2Client)
	if err != nil {
		return "", "", "", err
	}
	instanceID, err := startJumphost(acmKeyName, subnetID, securityGroupID, ec2Client)
	if err != nil {
		return "", "", "", err
	}
	jumphostIP, err := waitUntilJumphostStarted(instanceID, ec2Client)
	if err != nil {
		return "", "", "", err
	}
	if err := waitUnitlJumphostIsReachable(privateKey, jumphostIP); err != nil {
		return "", "", "", err
	}

	return instanceID, privateKey, jumphostIP, nil
}

func waitUnitlJumphostIsReachable(privateKey string, jumphostIP string) error {
	privateKeyFile, err := os.CreateTemp("", "jumphost-key-*.pem")
	defer os.Remove(privateKeyFile.Name()) // Clean up the temporary file after use
	if err != nil {
		return fmt.Errorf("failed to create temporary private key file: %w", err)
	}
	defer privateKeyFile.Close()

	if _, err := privateKeyFile.WriteString(privateKey); err != nil {
		return fmt.Errorf("failed to write private key to temporary file: %w", err)
	}
	if err := os.Chmod(privateKeyFile.Name(), 0o400); err != nil {
		return fmt.Errorf("failed to set permissions on temporary private key file: %w", err)
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
		return fmt.Errorf("jump host instance is not reachable via SSH at %s", jumphostIP)
	}
	log.Printf("Successfully connected to Jump Host Instance via SSH at %s", jumphostIP)
	return nil
}

func waitUntilJumphostStarted(instanceID string, ec2Client *ec2.EC2) (string, error) {
	describeInstancesInput := &ec2.DescribeInstancesInput{
		InstanceIds: []*string{&instanceID},
	}
	var jumphostIP string
	for range 6 {
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
		return "", fmt.Errorf("failed to retrieve public IP address of the jump host instance")
	}
	return jumphostIP, nil
}

func startJumphost(acmKeyName string, subnetID string, securityGroupID string, ec2Client *ec2.EC2) (string, error) {
	createInstanceInput := &ec2.RunInstancesInput{
		ImageId:      aws.String(DefaultJumphostAMIID),
		InstanceType: aws.String(ec2.InstanceTypeT3Micro),
		MinCount:     aws.Int64(1),
		MaxCount:     aws.Int64(1),
		KeyName:      aws.String(acmKeyName),
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
						Value: aws.String(DefaultCustomerTag),
					},
				},
			},
		},
	}
	runInstancesOutput, err := ec2Client.RunInstances(createInstanceInput)
	if err != nil {
		return "", fmt.Errorf("failed to create jump host instance: %w", err)
	}
	instanceID := *runInstancesOutput.Instances[0].InstanceId
	log.Printf("Created Jump Host Instance with ID: %s", instanceID)
	return instanceID, nil
}

func createJumphostSSHKeyPair(ec2Client *ec2.EC2) (string, string, error) {
	privateKey, publicKey, err := generateSSHKeyPair()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate SSH key pair: %w", err)
	}

	// Generate a random name for the key pair
	acmKeyName := fmt.Sprintf("jump-host-key-%s", strings.ToLower(rand.Text()[:8]))

	// Import the public key to AWS
	importKeyPairInput := &ec2.ImportKeyPairInput{
		KeyName:           aws.String(acmKeyName),
		PublicKeyMaterial: []byte(publicKey),
	}
	_, err = ec2Client.ImportKeyPair(importKeyPairInput)
	if err != nil {
		return "", "", fmt.Errorf("failed to import SSH key pair: %w", err)
	}
	log.Printf("Imported SSH key pair for jump host with name: %s", acmKeyName)
	return privateKey, acmKeyName, nil
}

func createJumphostSecurityGroup(vpcID string, ec2Client *ec2.EC2, ipAllowRanges []*ec2.IpRange) (string, error) {
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
						Value: aws.String(DefaultCustomerTag),
					},
				},
			},
		},
	}
	securityGroupOutput, err := ec2Client.CreateSecurityGroup(createSecurityGroupInput)
	if err != nil {
		return "", fmt.Errorf("failed to create security group: %w", err)
	}
	securityGroupID := *securityGroupOutput.GroupId
	log.Printf("Created Security Group with ID: %s", securityGroupID)

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
		return "", fmt.Errorf("failed to authorize security group ingress: %w", err)
	}
	log.Printf("Authorized inbound SSH traffic for Security Group ID: %s", securityGroupID)
	return securityGroupID, nil
}

func getMyPublicIP() (string, error) {
	var publicIP string
	if socksProxy := os.Getenv("SOCKS_PROXY"); socksProxy != "" {
		// Use SOCKS_PROXY to get the public IP address
		log.Printf("Using SOCKS_PROXY: %s\n", socksProxy)
		cmd := exec.Command("curl", "--socks5", socksProxy, "https://checkip.amazonaws.com")
		output, err := cmd.Output()
		if err != nil {
			return "", fmt.Errorf("failed to get public IP address using SOCKS_PROXY: %w", err)
		}
		publicIP = strings.TrimSpace(string(output))
	} else {
		// Directly get the public IP address
		resp, err := http.Get("https://checkip.amazonaws.com")
		if err != nil {
			return "", fmt.Errorf("failed to get public IP address: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to read response body: %w", err)
		}
		publicIP = strings.TrimSpace(string(body))
	}
	return publicIP, nil
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
	if err := os.Chmod(privateKeyFile.Name(), 0o400); err != nil {
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
