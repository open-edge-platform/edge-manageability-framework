// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/gruntwork-io/terratest/modules/testing"
	steps_aws "github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps/aws"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

const (
	DefaultRegion             = "us-west-2"
	DefaultCustomerTag        = "test-customer"
	DefaultJumphostAMIID      = "ami-0a605bc2ef5707a18" // Ubuntu 24.04 LTS in us-west-2
	DefaultJumphostSSHKeySize = 2048
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

// Creates VPC
// Returns VPC ID, public subnet IDs, private subnet IDs, jumphost private key, jumphost IP, and error if any
// Assume a S3 bucket is already exists with name `name`
func CreateVPC(t testing.TestingT, name string) (string, []string, []string, string, string, error) {
	var jumphostAllowList []string = make([]string, 0)
	publicIP, err := getMyPublicIP()
	if err == nil {
		myCIDR := fmt.Sprintf("%s/32", publicIP) // Allow this machine's IP
		jumphostAllowList = append(jumphostAllowList, myCIDR)
	}
	// Additionally allow the IPs from the environment variable JUMPHOST_IP_ALLOW_LIST
	if cidrAllowListStr := os.Getenv("JUMPHOST_IP_CIDR_ALLOW_LIST"); cidrAllowListStr != "" {
		for _, cidr := range strings.Split(cidrAllowListStr, ",") {
			cidr = strings.TrimSpace(cidr)
			if _, _, err := net.ParseCIDR(cidr); err != nil {
				continue // Skip invalid CIDR
			}
			if cidr != "" {
				jumphostAllowList = append(jumphostAllowList, cidr)
			}
		}
	}

	privateSSHKey, publicSSHKey, _ := GenerateSSHKeyPair()
	variables := steps_aws.VPCVariables{
		Region:             DefaultRegion,
		Name:               name,
		CidrBlock:          steps_aws.DefaultNetworkCIDR,
		EnableDnsHostnames: true,
		EnableDnsSupport:   true,
		PrivateSubnets: map[string]steps_aws.VPCSubnet{
			name + "priv-1": {
				Az:        "us-west-2a",
				CidrBlock: "10.250.0.0/22",
			},
			name + "priv-2": {
				Az:        "us-west-2b",
				CidrBlock: "10.250.4.0/22",
			},
			name + "priv-3": {
				Az:        "us-west-2c",
				CidrBlock: "10.250.8.0/22",
			},
		},
		PublicSubnets: map[string]steps_aws.VPCSubnet{
			name + "pub-1": {
				Az:        "us-west-2a",
				CidrBlock: "10.250.128.0/24",
			},
			name + "pub-2": {
				Az:        "us-west-2a",
				CidrBlock: "10.250.129.0/24",
			},
			name + "pub-3": {
				Az:        "us-west-2a",
				CidrBlock: "10.250.130.0/24",
			},
		},
		EndpointSGName:         name,
		JumphostIPAllowList:    jumphostAllowList,
		JumphostInstanceSSHKey: publicSSHKey,
		JumphostSubnet:         name + "pub-1",
		Production:             true,
		CustomerTag:            DefaultCustomerTag,
	}

	jsonData, err := json.Marshal(variables)
	if err != nil {
		t.Fatalf("Failed to marshal variables: %v", err)
	}
	tempFile, err := os.CreateTemp("", "variables-*.tfvar.json")
	if err != nil {
		t.Fatalf("Failed to create temporary file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	if _, err := tempFile.Write(jsonData); err != nil {
		t.Fatalf("Failed to write to temporary file: %v", err)
	}

	terraformOptions := terraform.WithDefaultRetryableErrors(t, &terraform.Options{
		TerraformDir: "../vpc",
		VarFiles:     []string{tempFile.Name()},
		BackendConfig: map[string]interface{}{
			"region": DefaultRegion,
			"bucket": name,
			"key":    "vpc.tfstate",
		},
		Reconfigure: true,
		Upgrade:     true,
	})
	terraform.InitAndApply(t, terraformOptions)
	vpcID, err := terraform.OutputE(t, terraformOptions, "vpc_id")
	if err != nil {
		require.NoError(t, err, "Failed to get VPC ID from Terraform output")
		return "", nil, nil, "", "", err
	}
	publicSubnetIDs, err := terraform.OutputListE(t, terraformOptions, "public_subnet_ids")
	if err != nil {
		require.NoError(t, err, "Failed to get public subnet IDs from Terraform output")
		return "", nil, nil, "", "", err
	}
	privateSubnetIDs, err := terraform.OutputListE(t, terraformOptions, "private_subnet_ids")
	if err != nil {
		require.NoError(t, err, "Failed to get private subnet IDs from Terraform output")
		return "", nil, nil, "", "", err
	}
	jumphostIP, err := terraform.OutputE(t, terraformOptions, "jumphost_ip")
	if err != nil {
		require.NoError(t, err, "Failed to get jumphost IP from Terraform output")
		return "", nil, nil, "", "", err
	}
	return vpcID, publicSubnetIDs, privateSubnetIDs, privateSSHKey, jumphostIP, err
}

// Deletes VPC and all its resources
func DeleteVPC(t testing.TestingT, name string) error {
	variables := steps_aws.VPCVariables{
		Region:             DefaultRegion,
		Name:               name,
		CidrBlock:          steps_aws.DefaultNetworkCIDR,
		EnableDnsHostnames: true,
		EnableDnsSupport:   true,
		PrivateSubnets: map[string]steps_aws.VPCSubnet{
			name + "priv-1": {
				Az:        "us-west-2a",
				CidrBlock: "10.250.0.0/22",
			},
			name + "priv-2": {
				Az:        "us-west-2b",
				CidrBlock: "10.250.4.0/22",
			},
			name + "priv-3": {
				Az:        "us-west-2c",
				CidrBlock: "10.250.8.0/22",
			},
		},
		PublicSubnets: map[string]steps_aws.VPCSubnet{
			name + "pub-1": {
				Az:        "us-west-2a",
				CidrBlock: "10.250.128.0/24",
			},
			name + "pub-2": {
				Az:        "us-west-2a",
				CidrBlock: "10.250.129.0/24",
			},
			name + "pub-3": {
				Az:        "us-west-2a",
				CidrBlock: "10.250.130.0/24",
			},
		},
		EndpointSGName:         name,
		JumphostIPAllowList:    []string{},
		JumphostInstanceSSHKey: "",
		JumphostSubnet:         name + "pub-1",
		Production:             true,
		CustomerTag:            DefaultCustomerTag,
	}
	jsonData, err := json.Marshal(variables)
	if err != nil {
		t.Fatalf("Failed to marshal variables: %v", err)
	}
	tempFile, err := os.CreateTemp("", "variables-*.tfvar.json")
	if err != nil {
		t.Fatalf("Failed to create temporary file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	if _, err := tempFile.Write(jsonData); err != nil {
		t.Fatalf("Failed to write to temporary file: %v", err)
	}
	terraformOptions := terraform.WithDefaultRetryableErrors(t, &terraform.Options{
		TerraformDir: "../vpc",
		VarFiles:     []string{tempFile.Name()},
		BackendConfig: map[string]interface{}{
			"region": DefaultRegion,
			"bucket": name,
			"key":    "vpc.tfstate",
		},
		Reconfigure: true,
		Upgrade:     true,
	})
	terraform.Destroy(t, terraformOptions)
	return nil
}

func WaitUnitlJumphostIsReachable(privateKey string, jumphostIP string) error {
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

func GenerateSSHKeyPair() (string, string, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, DefaultJumphostSSHKeySize)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %w", err)
	}

	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	}
	privateKeyString := string(pem.EncodeToMemory(privateKeyPEM))
	pub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", "", err
	}
	publicKeyString := string(ssh.MarshalAuthorizedKey(pub))
	return privateKeyString, publicKeyString, nil
}
