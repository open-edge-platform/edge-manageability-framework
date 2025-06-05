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
	"context"
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

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
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

// This function creates a VPC and three subnets in the specified AWS region.
func CreateVPC(region string, name string) (string, []string, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		return "", nil, fmt.Errorf("unable to load AWS SDK config: %w", err)
	}
	ec2Client := ec2.NewFromConfig(cfg)

	// Create VPC
	createVpcInput := &ec2.CreateVpcInput{
		CidrBlock: aws.String("10.250.0.0/16"),
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeVpc,
				Tags: []types.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(name),
					},
				},
			},
		},
	}
	vpcOutput, err := ec2Client.CreateVpc(context.Background(), createVpcInput)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create VPC: %w", err)
	}
	vpcID := *vpcOutput.Vpc.VpcId
	log.Printf("Created VPC with ID: %s", vpcID)

	// Create Internet Gateway
	createInternetGatewayInput := &ec2.CreateInternetGatewayInput{}
	igwOutput, err := ec2Client.CreateInternetGateway(context.Background(), createInternetGatewayInput)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create Internet Gateway: %w", err)
	}
	igwID := *igwOutput.InternetGateway.InternetGatewayId
	log.Printf("Created Internet Gateway with ID: %s", igwID)

	// Attach Internet Gateway to VPC
	attachIgwInput := &ec2.AttachInternetGatewayInput{
		InternetGatewayId: aws.String(igwID),
		VpcId:             aws.String(vpcID),
	}
	_, err = ec2Client.AttachInternetGateway(context.Background(), attachIgwInput)
	if err != nil {
		return "", nil, fmt.Errorf("failed to attach Internet Gateway to VPC: %w", err)
	}
	log.Printf("Attached Internet Gateway %s to VPC %s", igwID, vpcID)

	// Create Subnets
	subnetIDs := []string{}
	availabilityZones := []string{"a", "b", "c"}
	for i, zone := range availabilityZones {
		cidrBlock := fmt.Sprintf("10.250.%d.0/24", i)
		createSubnetInput := &ec2.CreateSubnetInput{
			CidrBlock:        aws.String(cidrBlock),
			VpcId:            aws.String(vpcID),
			AvailabilityZone: aws.String(region + zone),
		}
		subnetOutput, err := ec2Client.CreateSubnet(context.Background(), createSubnetInput)
		if err != nil {
			return "", nil, fmt.Errorf("failed to create subnet in zone %s: %w", zone, err)
		}
		subnetID := *subnetOutput.Subnet.SubnetId
		log.Printf("Created Subnet with ID: %s in zone %s", subnetID, zone)
		subnetIDs = append(subnetIDs, subnetID)
	}

	// Create Route Table
	createRouteTableInput := &ec2.CreateRouteTableInput{
		VpcId: aws.String(vpcID),
	}
	routeTableOutput, err := ec2Client.CreateRouteTable(context.Background(), createRouteTableInput)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create route table: %w", err)
	}
	routeTableID := *routeTableOutput.RouteTable.RouteTableId
	log.Printf("Created Route Table with ID: %s", routeTableID)

	// Create Route to Internet Gateway
	createRouteInput := &ec2.CreateRouteInput{
		RouteTableId:         aws.String(routeTableID),
		DestinationCidrBlock: aws.String("0.0.0.0/0"),
		GatewayId:            aws.String(igwID),
	}
	_, err = ec2Client.CreateRoute(context.Background(), createRouteInput)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create route to Internet Gateway: %w", err)
	}
	log.Printf("Created route to Internet Gateway %s in Route Table %s", igwID, routeTableID)

	// Associate Subnets with Route Table
	for _, subnetID := range subnetIDs {
		associateRouteTableInput := &ec2.AssociateRouteTableInput{
			RouteTableId: aws.String(routeTableID),
			SubnetId:     aws.String(subnetID),
		}
		_, err := ec2Client.AssociateRouteTable(context.Background(), associateRouteTableInput)
		if err != nil {
			return "", nil, fmt.Errorf("failed to associate subnet %s with route table: %w", subnetID, err)
		}
		log.Printf("Associated Subnet %s with Route Table %s", subnetID, routeTableID)
	}

	return vpcID, subnetIDs, nil
}

func DeleteVPC(region string, vpcID string) error {
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		return fmt.Errorf("unable to load AWS SDK config: %w", err)
	}
	ec2Client := ec2.NewFromConfig(cfg)
	deleteVpcInput := &ec2.DeleteVpcInput{
		VpcId: aws.String(vpcID),
	}
	// Retrieve and delete all subnets associated with the VPC
	describeSubnetsInput := &ec2.DescribeSubnetsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{vpcID},
			},
		},
	}
	subnetsOutput, err := ec2Client.DescribeSubnets(context.Background(), describeSubnetsInput)
	if err != nil {
		return fmt.Errorf("failed to describe subnets: %w", err)
	}

	for _, subnet := range subnetsOutput.Subnets {
		deleteSubnetInput := &ec2.DeleteSubnetInput{
			SubnetId: subnet.SubnetId,
		}
		_, err := ec2Client.DeleteSubnet(context.Background(), deleteSubnetInput)
		if err != nil {
			return fmt.Errorf("failed to delete subnet %s: %w", *subnet.SubnetId, err)
		}
		log.Printf("Deleted Subnet with ID: %s", *subnet.SubnetId)
	}
	_, err = ec2Client.DeleteVpc(context.Background(), deleteVpcInput)
	if err != nil {
		return fmt.Errorf("failed to delete VPC: %w", err)
	}
	log.Printf("Deleted VPC with ID: %s", vpcID)
	return nil
}

func CreateJumpHost(vpcID string, subnetID string, region string) (string, string, string, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		return "", "", "", fmt.Errorf("unable to load AWS SDK config: %w", err)
	}
	ec2Client := ec2.NewFromConfig(cfg)

	// Create a security group for the jump host
	createSecurityGroupInput := &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(fmt.Sprintf("jump-host-sg-%s", strings.ToLower(rand.Text()[:8]))),
		Description: aws.String("Security group for jump host"),
		VpcId:       aws.String(vpcID),
	}
	securityGroupOutput, err := ec2Client.CreateSecurityGroup(context.Background(), createSecurityGroupInput)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create security group: %w", err)
	}
	securityGroupID := *securityGroupOutput.GroupId
	log.Printf("Created Security Group with ID: %s", securityGroupID)

	// Get the public IP address of the machine running the test
	var publicIP string
	if socksProxy := os.Getenv("SOCKS_PROXY"); socksProxy != "" {
		// Use SOCKS_PROXY to get the public IP address
		fmt.Printf("Using SOCKS_PROXY: %s\n", socksProxy)
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
	fmt.Printf("Public IP of the machine running the test: %s\n", myCIDR)

	authorizeIngressInput := &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId: aws.String(securityGroupID),
		IpPermissions: []types.IpPermission{
			{
				IpProtocol: aws.String("-1"),                              // -1 allows all protocols
				IpRanges:   []types.IpRange{{CidrIp: aws.String(myCIDR)}}, // Allow traffic from this IP
			},
		},
	}
	_, err = ec2Client.AuthorizeSecurityGroupIngress(context.Background(), authorizeIngressInput)
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
	_, err = ec2Client.ImportKeyPair(context.Background(), importKeyPairInput)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to import SSH key pair: %w", err)
	}
	log.Printf("Imported SSH key pair for jump host with name: %s", keyName)

	// Create the jump host instance
	createInstanceInput := &ec2.RunInstancesInput{
		ImageId:      aws.String("ami-0a605bc2ef5707a18"), // Ubuntu 24.04 LTS in us-west-2
		InstanceType: types.InstanceTypeT3Micro,
		MinCount:     aws.Int32(1),
		MaxCount:     aws.Int32(1),
		KeyName:      aws.String(keyName),
		NetworkInterfaces: []types.InstanceNetworkInterfaceSpecification{
			{
				AssociatePublicIpAddress: aws.Bool(true),
				DeviceIndex:              aws.Int32(0),
				SubnetId:                 aws.String(subnetID),
				Groups:                   []string{securityGroupID},
			},
		},
	}
	runInstancesOutput, err := ec2Client.RunInstances(context.Background(), createInstanceInput)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create jump host instance: %w", err)
	}
	instanceID := *runInstancesOutput.Instances[0].InstanceId
	log.Printf("Created Jump Host Instance with ID: %s", instanceID)

	// Wait for the instance to be in the running state and retrieve its public IP address
	describeInstancesInput := &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	}

	var jumphostIP string
	for range 5 {
		log.Printf("Waiting for Jump Host Instance to be in running state...")
		time.Sleep(10 * time.Second)
		describeInstancesOutput, err := ec2Client.DescribeInstances(context.Background(), describeInstancesInput)
		if err != nil {
			fmt.Printf("Failed to describe instance: %v, continue...\n", err)
			continue
		}

		instance := describeInstancesOutput.Reservations[0].Instances[0]
		if instance.State.Name == types.InstanceStateNameRunning && instance.PublicIpAddress != nil {
			jumphostIP = *instance.PublicIpAddress
			log.Printf("Jump Host Instance is running with Public IP: %s", jumphostIP)
			break
		}
	}

	if jumphostIP == "" {
		return instanceID, privateKey, jumphostIP, fmt.Errorf("failed to retrieve public IP address of the jump host instance")
	}

	// Wait for the instance to be reachable via SSH
	var reachable bool = false
	for range 10 {
		var sshCmd *exec.Cmd
		if socksProxy := os.Getenv("SOCKS_PROXY"); socksProxy != "" {
			sshCmd = exec.Command("ssh", "-T", "-o", fmt.Sprintf("ProxyCommand=nc -x %s %%h %%p", socksProxy), "-o", "StrictHostKeyChecking=no", "-i", "/dev/stdin", fmt.Sprintf("ubuntu@%s", jumphostIP))
		} else {
			sshCmd = exec.Command("ssh", "-T", "-o", "StrictHostKeyChecking=no", "-i", "/dev/stdin", fmt.Sprintf("ubuntu@%s", jumphostIP))
		}
		sshCmd.Stdin = strings.NewReader(privateKey)
		if err := sshCmd.Run(); err != nil {
			continue
		}
		reachable = true
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
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		return fmt.Errorf("unable to load AWS SDK config: %w", err)
	}
	ec2Client := ec2.NewFromConfig(cfg)

	// Terminate the jump host instance
	terminateInstancesInput := &ec2.TerminateInstancesInput{
		InstanceIds: []string{instanceID},
	}
	_, err = ec2Client.TerminateInstances(context.Background(), terminateInstancesInput)
	if err != nil {
		return fmt.Errorf("failed to terminate jump host instance: %w", err)
	}
	log.Printf("Terminated Jump Host Instance with ID: %s", instanceID)

	return nil
}

func StartSshuttle(jumphostIP string, jumphostKey string, remoteCIDRBlock string) error {
	// Create a temporary file for the private key
	privateKeyFile, err := os.CreateTemp("", "jumphost-key-*.pem")
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
	if err := os.Chmod(privateKeyFile.Name(), 0600); err != nil {
		return fmt.Errorf("failed to set permissions on temporary private key file: %w", err)
	}
	// Construct the sshuttle command
	var cmd *exec.Cmd
	if socksProxy := os.Getenv("SOCKS_PROXY"); socksProxy != "" {
		cmd = exec.Command("sshuttle", "--pidfile", "/tmp/sshuttle.pid", "-D", "-e", fmt.Sprintf("ssh -o ProxyCommand='nc -x %s %%h %%p' -i %s -o StrictHostKeyChecking=no", socksProxy, privateKeyFile.Name()), "-r", fmt.Sprintf("ubuntu@%s", jumphostIP), remoteCIDRBlock)
	} else {
		cmd = exec.Command("sshuttle", "--pidfile", "/tmp/sshuttle.pid", "-D", "-r", fmt.Sprintf("ubuntu@%s", jumphostIP), "--ssh-cmd", fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=no", privateKeyFile.Name()), remoteCIDRBlock)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start sshuttle command: %w", err)
	}

	// We will let the sshuttle process run in the background.
	return nil
}

func StopSshuttle() error {
	// Read the PID from the sshuttle PID file
	pidFile := "/tmp/sshuttle.pid"
	pidData, err := os.ReadFile(pidFile)
	if err != nil {
		return fmt.Errorf("failed to read sshuttle PID file: %w", err)
	}

	// Convert the PID to an integer
	pid := strings.TrimSpace(string(pidData))
	cmd := exec.Command("kill", pid)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to terminate sshuttle process with PID %s: %w", pid, err)
	}

	// Remove the PID file
	if err := os.Remove(pidFile); err != nil {
		return fmt.Errorf("failed to remove sshuttle PID file: %w", err)
	}

	log.Println("Stopped sshuttle successfully")
	return nil
}
