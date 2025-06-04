// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package aws_iac_test

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/iam"
)

const (
	vpcSuffix     = "-vpc"
	iamSuffix     = "-eks-role"
	nodeIAMSuffix = "-node-role"
)

func setupIAMRole(name string, region string) (string, error) {
	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	iamClient := iam.New(sess)

	roleInput := &iam.CreateRoleInput{
		RoleName: aws.String(name + iamSuffix),
		AssumeRolePolicyDocument: aws.String(`{
			"Version": "2012-10-17",
			"Statement": [
				{
				"Effect": "Allow",
				"Principal": {
					"Service": "eks.amazonaws.com"
				},
				"Action": "sts:AssumeRole"
				}
			]
			}`),
	}
	roleOutput, err := iamClient.CreateRole(roleInput)
	if err != nil {
		return "", fmt.Errorf("failed to create IAM role: %w", err)
	}
	rolePolicies := []string{
		"arn:aws:iam::aws:policy/AmazonEKSClusterPolicy",
		"arn:aws:iam::aws:policy/AmazonEKSServicePolicy",
	}

	for _, rolePolicy := range rolePolicies {
		_, err = iamClient.AttachRolePolicy(&iam.AttachRolePolicyInput{
			RoleName:  aws.String(name + iamSuffix),
			PolicyArn: aws.String(rolePolicy),
		})
		if err != nil {
			return "", fmt.Errorf("failed to attach policy %s to IAM role: %w", rolePolicy, err)
		}
	}
	return *roleOutput.Role.Arn, nil
}

func deleteIAMRole(roleName string, region string) error {
	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	iamClient := iam.New(sess)

	_, err = iamClient.DeleteRole(&iam.DeleteRoleInput{
		RoleName: aws.String(roleName),
	})
	if err != nil {
		return fmt.Errorf("failed to delete IAM role: %w", err)
	}
	fmt.Printf("IAM role %s deleted successfully\n", roleName)
	return nil
}

func setupVPC(name string, region string) (string, error) {
	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	ec2Client := ec2.New(sess)

	newVPC, err := ec2Client.CreateVpc(&ec2.CreateVpcInput{
		CidrBlock: aws.String("192.168.0.0/16"),
		TagSpecifications: []*ec2.TagSpecification{
			{
				ResourceType: aws.String("vpc"),
				Tags: []*ec2.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(name + vpcSuffix),
					},
					{
						Key:   aws.String("Environment"),
						Value: aws.String("Test"),
					},
				},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create VPC: %w", err)
	}
	return *newVPC.Vpc.VpcId, nil
}

func deleteVPC(vpcId string, region string) error {
	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	ec2Client := ec2.New(sess)

	// Delete all subnets associated with the VPC
	subnets, err := ec2Client.DescribeSubnets(&ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []*string{aws.String(vpcId)},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to describe subnets: %w", err)
	}
	for _, subnet := range subnets.Subnets {
		fmt.Println("Deleting subnet:", *subnet.SubnetId)
		_, err = ec2Client.DeleteSubnet(&ec2.DeleteSubnetInput{
			SubnetId: subnet.SubnetId,
		})
		if err != nil {
			fmt.Printf("Error deleting subnet %s: %v\n", *subnet.SubnetId, err)
			return fmt.Errorf("failed to delete subnet: %w", err)
		}
		fmt.Printf("Subnet %s deleted successfully\n", *subnet.SubnetId)
	}

	// Delete the VPC
	fmt.Println("Deleting VPC:", vpcId)
	_, err = ec2Client.DeleteVpc(&ec2.DeleteVpcInput{
		VpcId: aws.String(vpcId),
	})
	if err != nil {
		fmt.Printf("Error deleting VPC %s: %v\n", vpcId, err)
		return fmt.Errorf("failed to delete VPC: %w", err)
	}
	fmt.Printf("VPC %s deleted successfully\n", vpcId)
	return nil
}

func setupSubnets(vpcId string, region string) ([]string, error) {
	subnets := []string{}
	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		return subnets, fmt.Errorf("failed to create session: %w", err)
	}
	ec2Client := ec2.New(sess)

	subnet1, err := createSubnet("subnet-1", "192.168.0.0/18", ec2Client, vpcId, region, "usw2-az1")
	if err != nil {
		return subnets, fmt.Errorf("failed to create subnet 1: %w", err)
	}
	subnets = append(subnets, subnet1)

	subnet2, err := createSubnet("subnet-2", "192.168.64.0/18", ec2Client, vpcId, region, "usw2-az2")
	if err != nil {
		return subnets, fmt.Errorf("failed to create subnet 2: %w", err)
	}
	subnets = append(subnets, subnet2)

	return subnets, nil
}

func createSubnet(name string, cidr string, ec2Client *ec2.EC2, vpcId string, region string, az string) (string, error) {
	subnetInput := &ec2.CreateSubnetInput{
		VpcId:              aws.String(vpcId),
		CidrBlock:          aws.String(cidr),
		AvailabilityZoneId: aws.String(az),
		TagSpecifications: []*ec2.TagSpecification{
			{
				ResourceType: aws.String("subnet"),
				Tags: []*ec2.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(name),
					},
					{
						Key:   aws.String("Environment"),
						Value: aws.String("Test"),
					},
				},
			},
		},
	}

	subnetOutput, err := ec2Client.CreateSubnet(subnetInput)
	if err != nil {
		return "", fmt.Errorf("failed to create subnet: %w", err)
	}
	return *subnetOutput.Subnet.SubnetId, nil
}

func setupEKS(clusterName string, region string, subnets []string, roleARN string) error {
	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	eksClient := eks.New(sess)

	clusterInput := &eks.CreateClusterInput{
		Name:    &clusterName,
		RoleArn: aws.String(roleARN),
		ResourcesVpcConfig: &eks.VpcConfigRequest{
			SubnetIds: aws.StringSlice(subnets),
		},
	}

	_, err = eksClient.CreateCluster(clusterInput)
	if err != nil {
		return fmt.Errorf("failed to create EKS cluster: %w", err)
	}
	return nil
}

func deleteEKS(clusterName string, region string) error {
	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	eksClient := eks.New(sess)

	_, err = eksClient.DeleteCluster(&eks.DeleteClusterInput{
		Name: aws.String(clusterName),
	})
	if err != nil {
		return fmt.Errorf("failed to delete EKS cluster: %w", err)
	}
	fmt.Printf("EKS cluster %s deleted successfully\n", clusterName)
	return nil
}

func CreateTestEKSCluster(clusterName string, region string) (string, string, error) {
	roleARN, err := setupIAMRole(clusterName, region)
	if err != nil {
		fmt.Printf("Error setting up IAM role: %v\n", err)
		return "", "", err
	}

	vpcId, err := setupVPC(clusterName, region)
	if err != nil {
		fmt.Printf("Error setting up subnets: %v\n", err)
		err = deleteIAMRole(clusterName+iamSuffix, region)
		if err != nil {
			fmt.Printf("Error deleting IAM role: %v\n", err)
		}
		return "", "", err
	}

	subnets, err := setupSubnets(vpcId, region)
	if err != nil {
		fmt.Printf("Error setting up subnets: %v\n", err)
		err = deleteVPC(vpcId, region)
		if err != nil {
			fmt.Printf("Error deleting VPC: %v\n", err)
		}
		err = deleteIAMRole(clusterName+iamSuffix, region)
		if err != nil {
			fmt.Printf("Error deleting IAM role: %v\n", err)
		}
		return "", "", err
	}
	err = setupEKS(clusterName, region, subnets, roleARN)
	if err != nil {
		fmt.Printf("Error setting up EKS cluster: %v\n", err)
		err = deleteVPC(vpcId, region)
		if err != nil {
			fmt.Printf("Error deleting VPC: %v\n", err)
		}
		err = deleteIAMRole(clusterName+iamSuffix, region)
		if err != nil {
			fmt.Printf("Error deleting IAM role: %v\n", err)
		}
		return "", "", err
	}
	return clusterName, vpcId, nil
}

func WaitForClusterInActiveState(clusterName string, region string, timeout time.Duration) error {
	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	eksClient := eks.New(sess)

	for i := 0 * time.Second; i < timeout; i += 30 * time.Second {
		describeInput := &eks.DescribeClusterInput{
			Name: aws.String(clusterName),
		}
		describeOutput, err := eksClient.DescribeCluster(describeInput)
		if err != nil {
			return fmt.Errorf("failed to describe EKS cluster: %w", err)
		}

		if describeOutput.Cluster.Status == nil || *describeOutput.Cluster.Status == eks.ClusterStatusActive {
			fmt.Printf("EKS cluster %s is in active state\n", clusterName)
			return nil
		}

		fmt.Printf("Waiting for EKS cluster %s to be active...\n", clusterName)
		time.Sleep(30 * time.Second) // Wait before checking again
	}
	return fmt.Errorf("EKS cluster %s did not become active within the timeout period", clusterName)
}

func DeleteTestEKSCluster(clusterName string, vpcId string, region string) error {
	fmt.Println("Deleting EKS")
	err := deleteEKS(clusterName, region)
	if err != nil {
		return fmt.Errorf("failed to delete EKS cluster: %w", err)
	}
	time.Sleep(30 * time.Second) // Wait for EKS cluster deletion to propagate
	fmt.Println("Deleting VPC")
	err = deleteVPC(vpcId, region)
	if err != nil {
		return fmt.Errorf("failed to delete VPC: %w", err)
	}
	fmt.Println("Deleting IAM role")
	err = deleteIAMRole(clusterName+iamSuffix, region)
	if err != nil {
		return fmt.Errorf("failed to delete IAM role: %w", err)
	}

	fmt.Printf("Successfully deleted EKS cluster %s and associated resources", clusterName)

	return nil
}
