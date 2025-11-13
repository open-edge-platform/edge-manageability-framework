// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

type SecretsManagerAPI interface {
	GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
	PutSecretValue(ctx context.Context, params *secretsmanager.PutSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error)
}

type AWSSM struct {
	Name string
	API  SecretsManagerAPI
}

func NewAWSSM(name string, region string) *AWSSM {
	// if region is empty then set to us-west-2
	if region == "" {
		region = "us-west-2"
	}
	
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
	)
	if err != nil {
		panic(fmt.Sprintf("not able to configure aws session: %v", err))
	}
	
	svc := secretsmanager.NewFromConfig(cfg)
	return &AWSSM{
		Name: name,
		API:  svc,
	}
}

// SaveSecret saves the secret to AWS Secrets Manager
func (f *AWSSM) SaveSecret(secret string) error {
	input := &secretsmanager.PutSecretValueInput{
		SecretId:     aws.String(f.Name),
		SecretString: aws.String(secret),
	}
	_, err := f.API.PutSecretValue(context.TODO(), input)
	if err != nil {
		return err
	}
	return nil
}

// GetSecret retrieves the secret from AWS Secrets Manager
func (f *AWSSM) GetSecret(name string) (string, error) {
	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(f.Name),
	}
	result, err := f.API.GetSecretValue(context.TODO(), input)
	if err != nil {
		return "", err
	}
	return *result.SecretString, nil
}
