// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"
)

const (
	dummyCert = "MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAn"
)

func TestValidateOrchName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
			errMsg:  "orchestrator name cannot be empty",
		},
		{
			name:    "too long",
			input:   "abcdefghijklmnop", // 16 chars
			wantErr: true,
			errMsg:  "orchestrator name must be less than 16 characters",
		},
		{
			name:    "contains uppercase",
			input:   "abcDef",
			wantErr: true,
			errMsg:  "orchestrator name must be all lower case letters or digits",
		},
		{
			name:    "contains special char",
			input:   "abc_def",
			wantErr: true,
			errMsg:  "orchestrator name must be all lower case letters or digits",
		},
		{
			name:    "contains dash",
			input:   "abc-def",
			wantErr: true,
			errMsg:  "orchestrator name must be all lower case letters or digits",
		},
		{
			name:    "contains digit",
			input:   "abc123",
			wantErr: false,
		},
		{
			name:    "all lowercase",
			input:   "orchestrator",
			wantErr: false,
		},
		{
			name:    "max allowed length",
			input:   "abcdefghijklmno", // 15 chars
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOrchName(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("expected error message %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestValidateParentDomain(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid domain with one dot",
			input:   "example.com",
			wantErr: false,
		},
		{
			name:    "valid domain with two dot",
			input:   "subdomain.example.com",
			wantErr: false,
		},
		{
			name:    "valid domain with dash",
			input:   "my-domain.com",
			wantErr: false,
		},
		{
			name:    "valid domain with digits",
			input:   "abc123.com",
			wantErr: false,
		},
		{
			name:    "valid domain with multiple dashes",
			input:   "a-b-c-d.com",
			wantErr: false,
		},
		{
			name:    "valid domain with multiple dots",
			input:   "abc.def",
			wantErr: false,
		},
		{
			name:    "invalid domain with uppercase",
			input:   "Example.com",
			wantErr: true,
			errMsg:  "parent domain must be all lower case letters, digits, or '.'",
		},
		{
			name:    "invalid domain with underscore",
			input:   "abc_def.com",
			wantErr: true,
			errMsg:  "parent domain must be all lower case letters, digits, or '.'",
		},
		{
			name:    "invalid domain with trailing dot",
			input:   "abc.",
			wantErr: true,
			errMsg:  "parent domain must be all lower case letters, digits, or '.'",
		},
		{
			name:    "invalid domain with leading dot",
			input:   ".com",
			wantErr: true,
			errMsg:  "parent domain must be all lower case letters, digits, or '.'",
		},
		{
			name:    "invalid domain with no dot",
			input:   "examplecom",
			wantErr: true,
			errMsg:  "parent domain must be all lower case letters, digits, or '.'",
		},
		{
			name:    "invalid domain with special char",
			input:   "abc@def.com",
			wantErr: true,
			errMsg:  "parent domain must be all lower case letters, digits, or '.'",
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
			errMsg:  "parent domain must be all lower case letters, digits, or '.'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateParentDomain(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("expected error message %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestValidateAdminEmail(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid simple email",
			input:   "user@example.com",
			wantErr: false,
		},
		{
			name:    "valid email with dot",
			input:   "first.last@example.com",
			wantErr: false,
		},
		{
			name:    "valid email with plus",
			input:   "user+test@example.com",
			wantErr: false,
		},
		{
			name:    "valid email with dash",
			input:   "user-test@example.com",
			wantErr: false,
		},
		{
			name:    "valid email with underscore",
			input:   "user_test@example.com",
			wantErr: false,
		},
		{
			name:    "valid email with digits",
			input:   "user123@example123.com",
			wantErr: false,
		},
		{
			name:    "valid email with subdomain",
			input:   "user@mail.example.com",
			wantErr: false,
		},
		{
			name:    "invalid email missing @",
			input:   "userexample.com",
			wantErr: true,
			errMsg:  "admin email must be a valid email address",
		},
		{
			name:    "invalid email missing domain",
			input:   "user@",
			wantErr: true,
			errMsg:  "admin email must be a valid email address",
		},
		{
			name:    "invalid email missing username",
			input:   "@example.com",
			wantErr: true,
			errMsg:  "admin email must be a valid email address",
		},
		{
			name:    "invalid email with uppercase",
			input:   "User@Example.com",
			wantErr: true,
			errMsg:  "admin email must be a valid email address",
		},
		{
			name:    "invalid email with invalid char",
			input:   "user!@example.com",
			wantErr: true,
			errMsg:  "admin email must be a valid email address",
		},
		{
			name:    "invalid email with short tld",
			input:   "user@example.c",
			wantErr: true,
			errMsg:  "admin email must be a valid email address",
		},
		{
			name:    "invalid email with no tld",
			input:   "user@example",
			wantErr: true,
			errMsg:  "admin email must be a valid email address",
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
			errMsg:  "admin email must be a valid email address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAdminEmail(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("expected error message %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestValidateAwsRegion(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid region us-west-2",
			input:   "us-west-2",
			wantErr: false,
		},
		{
			name:    "valid region eu-central-1",
			input:   "eu-central-1",
			wantErr: false,
		},
		{
			name:    "valid region ap-south-1",
			input:   "ap-south-1",
			wantErr: false,
		},
		{
			name:    "invalid region with uppercase",
			input:   "US-WEST-2",
			wantErr: true,
			errMsg:  "region must follow the format '^[a-z]+-[a-z]+-\\d$', e.g., 'us-west-2'",
		},
		{
			name:    "invalid region missing dash",
			input:   "uswest2",
			wantErr: true,
			errMsg:  "region must follow the format '^[a-z]+-[a-z]+-\\d$', e.g., 'us-west-2'",
		},
		{
			name:    "invalid region with two digit ending",
			input:   "us-west-12",
			wantErr: true,
			errMsg:  "region must follow the format '^[a-z]+-[a-z]+-\\d$', e.g., 'us-west-2'",
		},
		{
			name:    "invalid region with trailing dash",
			input:   "us-west-",
			wantErr: true,
			errMsg:  "region must follow the format '^[a-z]+-[a-z]+-\\d$', e.g., 'us-west-2'",
		},
		{
			name:    "invalid region with leading dash",
			input:   "-us-west-2",
			wantErr: true,
			errMsg:  "region must follow the format '^[a-z]+-[a-z]+-\\d$', e.g., 'us-west-2'",
		},
		{
			name:    "invalid region with special char",
			input:   "us-west@2",
			wantErr: true,
			errMsg:  "region must follow the format '^[a-z]+-[a-z]+-\\d$', e.g., 'us-west-2'",
		},
		{
			name:    "invalid region empty string",
			input:   "",
			wantErr: true,
			errMsg:  "region must follow the format '^[a-z]+-[a-z]+-\\d$', e.g., 'us-west-2'",
		},
		{
			name:    "invalid region with space",
			input:   "us-west 2",
			wantErr: true,
			errMsg:  "region must follow the format '^[a-z]+-[a-z]+-\\d$', e.g., 'us-west-2'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAwsRegion(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("expected error message %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestValidateAwsCustomTag(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "empty string",
			input:   "",
			wantErr: false,
		},
		{
			name:    "simple tag",
			input:   "Environment=Production",
			wantErr: false,
		},
		{
			name:    "tag with spaces",
			input:   "Owner = John Doe",
			wantErr: false,
		},
		{
			name:    "tag with special characters",
			input:   "Project=My-App_123",
			wantErr: false,
		},
		{
			name:    "tag with unicode",
			input:   "部署=生产",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAwsCustomTag(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestValidateCacheRegistry(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "empty string",
			input:   "",
			wantErr: false,
		},
		{
			name:    "simple registry url",
			input:   "myregistry.example.com",
			wantErr: false,
		},
		{
			name:    "registry with port",
			input:   "myregistry.example.com:5000",
			wantErr: false,
		},
		{
			name:    "registry with path",
			input:   "myregistry.example.com/myrepo",
			wantErr: false,
		},
		{
			name:    "docker hub style",
			input:   "docker.io/library/ubuntu",
			wantErr: false,
		},
		{
			name:    "localhost registry",
			input:   "localhost:5000",
			wantErr: false,
		},
		{
			name:    "IP address registry",
			input:   "192.168.1.100:5000",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCacheRegistry(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestValidateAwsJumpHostWhitelist(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "empty string",
			input:   "",
			wantErr: false,
		},
		{
			name:    "single IP",
			input:   "192.168.1.1",
			wantErr: false,
		},
		{
			name:    "multiple IPs comma separated",
			input:   "192.168.1.1,10.0.0.2",
			wantErr: false,
		},
		{
			name:    "IP with spaces",
			input:   " 192.168.1.1 , 10.0.0.2 ",
			wantErr: false,
		},
		{
			name:    "hostname",
			input:   "jump.example.com",
			wantErr: false,
		},
		{
			name:    "multiple hostnames",
			input:   "jump1.example.com,jump2.example.com",
			wantErr: false,
		},
		{
			name:    "mix of IPs and hostnames",
			input:   "192.168.1.1,jump.example.com",
			wantErr: false,
		},
		{
			name:    "wildcard",
			input:   "*",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAwsJumpHostWhitelist(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestValidateAwsVpcId(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty string",
			input:   "",
			wantErr: false,
		},
		{
			name:    "valid vpc id lowercase",
			input:   "vpc-1234abcd",
			wantErr: false,
		},
		{
			name:    "valid vpc id all digits",
			input:   "vpc-12345678",
			wantErr: false,
		},
		{
			name:    "valid vpc id all hex",
			input:   "vpcabcdef0",
			wantErr: true, // missing dash and not 8 chars after dash
			errMsg:  "VPC ID must follow the format '^vpc-[0-9a-f]{8}$', e.g., 'vpc-12345678'",
		},
		{
			name:    "invalid vpc id uppercase",
			input:   "vpc-1234ABCD",
			wantErr: true,
			errMsg:  "VPC ID must follow the format '^vpc-[0-9a-f]{8}$', e.g., 'vpc-12345678'",
		},
		{
			name:    "invalid vpc id too short",
			input:   "vpc-1234abc",
			wantErr: true,
			errMsg:  "VPC ID must follow the format '^vpc-[0-9a-f]{8}$', e.g., 'vpc-12345678'",
		},
		{
			name:    "invalid vpc id too long",
			input:   "vpc-1234abcdef",
			wantErr: true,
			errMsg:  "VPC ID must follow the format '^vpc-[0-9a-f]{8}$', e.g., 'vpc-12345678'",
		},
		{
			name:    "invalid vpc id missing dash",
			input:   "vpc12345678",
			wantErr: true,
			errMsg:  "VPC ID must follow the format '^vpc-[0-9a-f]{8}$', e.g., 'vpc-12345678'",
		},
		{
			name:    "invalid vpc id wrong prefix",
			input:   "vcp-12345678",
			wantErr: true,
			errMsg:  "VPC ID must follow the format '^vpc-[0-9a-f]{8}$', e.g., 'vpc-12345678'",
		},
		{
			name:    "invalid vpc id with special char",
			input:   "vpc-1234abc!",
			wantErr: true,
			errMsg:  "VPC ID must follow the format '^vpc-[0-9a-f]{8}$', e.g., 'vpc-12345678'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAwsVpcId(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("expected error message %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestValidateAwsEksDnsIp(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty string",
			input:   "",
			wantErr: false,
		},
		{
			name:    "valid IP",
			input:   "10.100.0.10",
			wantErr: false,
		},
		{
			name:    "valid IP with leading zeros",
			input:   "010.001.000.010",
			wantErr: false,
		},
		{
			name:    "invalid IP with too few octets",
			input:   "10.0.1",
			wantErr: true,
			errMsg:  "EKS DNS IP must follow the format '^([0-9]{1,3}\\.){3}[0-9]{1,3}$', e.g., '",
		},
		{
			name:    "invalid IP with too many octets",
			input:   "10.0.1.2.3",
			wantErr: true,
			errMsg:  "EKS DNS IP must follow the format '^([0-9]{1,3}\\.){3}[0-9]{1,3}$', e.g., '",
		},
		{
			name:    "invalid IP with letters",
			input:   "10.0.a.1",
			wantErr: true,
			errMsg:  "EKS DNS IP must follow the format '^([0-9]{1,3}\\.){3}[0-9]{1,3}$', e.g., '",
		},
		{
			name:    "invalid IP with special chars",
			input:   "10.0.0@1",
			wantErr: true,
			errMsg:  "EKS DNS IP must follow the format '^([0-9]{1,3}\\.){3}[0-9]{1,3}$', e.g., '",
		},
		{
			name:    "invalid IP with space",
			input:   "10.0.0. 1",
			wantErr: true,
			errMsg:  "EKS DNS IP must follow the format '^([0-9]{1,3}\\.){3}[0-9]{1,3}$', e.g., '",
		},
		{
			name:    "invalid IP with negative number",
			input:   "10.0.-1.1",
			wantErr: true,
			errMsg:  "EKS DNS IP must follow the format '^([0-9]{1,3}\\.){3}[0-9]{1,3}$', e.g., '",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAwsEksDnsIp(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("expected error message %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestValidateProxy(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "empty string",
			input:   "",
			wantErr: false,
		},
		{
			name:    "http proxy",
			input:   "http://proxy.example.com:8080",
			wantErr: false,
		},
		{
			name:    "https proxy",
			input:   "https://proxy.example.com:8443",
			wantErr: false,
		},
		{
			name:    "proxy with user info",
			input:   "http://user:pass@proxy.example.com:8080",
			wantErr: true,
		},
		{
			name:    "proxy with IP address",
			input:   "http://192.168.1.100:3128",
			wantErr: false,
		},
		{
			name:    "proxy with no port",
			input:   "http://proxy.example.com",
			wantErr: false,
		},
		{
			name:    "invalid proxy string",
			input:   "not a proxy",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateProxy(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestValidateNoProxy(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty string",
			input:   "",
			wantErr: false,
		},
		{
			name:    "single valid IP",
			input:   "192.168.1.1",
			wantErr: false,
		},
		{
			name:    "single valid CIDR",
			input:   "10.0.0.0/24",
			wantErr: false,
		},
		{
			name:    "multiple valid IPs and CIDRs",
			input:   "192.168.1.1,10.0.0.0/24,172.16.0.1",
			wantErr: false,
		},
		{
			name:    "valid domain",
			input:   ".example.com",
			wantErr: false,
		},
		{
			name:    "valid domain without leading dot",
			input:   "example.com",
			wantErr: false,
		},
		{
			name:    "multiple valid domains and IPs",
			input:   "example.com,192.168.1.1,.internal.local",
			wantErr: false,
		},
		{
			name:    "valid IP with spaces",
			input:   " 192.168.1.1 ",
			wantErr: false,
		},
		{
			name:    "valid domain with spaces",
			input:   " .example.com ",
			wantErr: false,
		},
		{
			name:    "invalid IP format",
			input:   "192.168.1",
			wantErr: true,
			errMsg:  "invalid no_proxy entry: 192.168.1",
		},
		{
			name:    "invalid CIDR mask non-numeric",
			input:   "10.0.0.0/abc",
			wantErr: true,
			errMsg:  "invalid no_proxy entry: 10.0.0.0/abc",
		},
		{
			name:    "invalid CIDR mask out of range",
			input:   "10.0.0.0/33",
			wantErr: true,
			errMsg:  "invalid CIDR mask in no_proxy entry: 10.0.0.0/33",
		},
		{
			name:    "invalid IP octet out of range",
			input:   "256.0.0.1",
			wantErr: true,
			errMsg:  "invalid IP in no_proxy entry: 256.0.0.1",
		},
		{
			name:    "invalid entry with special char",
			input:   "abc@def.com",
			wantErr: true,
			errMsg:  "invalid no_proxy entry: abc@def.com",
		},
		{
			name:    "invalid entry with empty between commas",
			input:   "example.com,,192.168.1.1",
			wantErr: false,
		},
		{
			name:    "invalid entry with only spaces",
			input:   "   ",
			wantErr: false,
		},
		{
			name:    "invalid IP with negative octet",
			input:   "10.0.-1.1",
			wantErr: true,
			errMsg:  "invalid no_proxy entry: 10.0.-1.1",
		},
		{
			name:    "invalid domain with uppercase",
			input:   "Example.com",
			wantErr: true,
			errMsg:  "invalid no_proxy entry: Example.com",
		},
		{
			name:    "valid domain with dash",
			input:   "my-domain.com",
			wantErr: false,
		},
		{
			name:    "valid domain with numbers",
			input:   "abc123.com",
			wantErr: false,
		},
		{
			name:    "invalid domain with underscore",
			input:   "abc_def.com",
			wantErr: true,
			errMsg:  "invalid no_proxy entry: abc_def.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNoProxy(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("expected error message %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestValidateTlsCert(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty string",
			input:   "",
			wantErr: false,
		},
		{
			name:    "valid PEM certificate single line",
			input:   "-----BEGIN CERTIFICATE-----\n" + dummyCert + "\n-----END CERTIFICATE-----",
			wantErr: false,
		},
		{
			name:    "valid PEM certificate multiline",
			input:   "-----BEGIN CERTIFICATE-----\n" + dummyCert + "\n" + dummyCert + "\n-----END CERTIFICATE-----",
			wantErr: false,
		},
		{
			name:    "missing BEGIN line",
			input:   dummyCert + "\n-----END CERTIFICATE-----",
			wantErr: true,
			errMsg:  "TLS certificate must be in PEM format",
		},
		{
			name:    "missing END line",
			input:   "-----BEGIN CERTIFICATE-----\n" + dummyCert,
			wantErr: true,
			errMsg:  "TLS certificate must be in PEM format",
		},
		{
			name:    "wrong header",
			input:   "-----BEGIN CERT-----\n" + dummyCert + "\n-----END CERTIFICATE-----",
			wantErr: true,
			errMsg:  "TLS certificate must be in PEM format",
		},
		{
			name:    "wrong footer",
			input:   "-----BEGIN CERTIFICATE-----\n" + dummyCert + "\n-----END CERT-----",
			wantErr: true,
			errMsg:  "TLS certificate must be in PEM format",
		},
		{
			name:    "no newlines",
			input:   "-----BEGIN CERTIFICATE-----" + dummyCert + "-----END CERTIFICATE-----",
			wantErr: true,
			errMsg:  "TLS certificate must be in PEM format",
		},
		{
			name:    "extra text before",
			input:   "extra\n-----BEGIN CERTIFICATE-----\n" + dummyCert + "\n-----END CERTIFICATE-----",
			wantErr: true,
			errMsg:  "TLS certificate must be in PEM format",
		},
		{
			name:    "extra text after",
			input:   "-----BEGIN CERTIFICATE-----\n" + dummyCert + "\n-----END CERTIFICATE-----\nextra",
			wantErr: true,
			errMsg:  "TLS certificate must be in PEM format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTlsCert(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("expected error message %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestValidateTlsKey(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty string",
			input:   "",
			wantErr: false,
		},
		{
			name:    "valid PEM private key single line",
			input:   "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQD\n-----END PRIVATE KEY-----",
			wantErr: false,
		},
		{
			name:    "valid PEM private key multiline",
			input:   "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQD\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQD\n-----END PRIVATE KEY-----",
			wantErr: false,
		},
		{
			name:    "missing BEGIN line",
			input:   "MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQD\n-----END PRIVATE KEY-----",
			wantErr: true,
			errMsg:  "TLS key must be in PEM format",
		},
		{
			name:    "missing END line",
			input:   "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQD",
			wantErr: true,
			errMsg:  "TLS key must be in PEM format",
		},
		{
			name:    "wrong header",
			input:   "-----BEGIN KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQD\n-----END PRIVATE KEY-----",
			wantErr: true,
			errMsg:  "TLS key must be in PEM format",
		},
		{
			name:    "wrong footer",
			input:   "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQD\n-----END KEY-----",
			wantErr: true,
			errMsg:  "TLS key must be in PEM format",
		},
		{
			name:    "no newlines",
			input:   "-----BEGIN PRIVATE KEY-----MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQD-----END PRIVATE KEY-----",
			wantErr: true,
			errMsg:  "TLS key must be in PEM format",
		},
		{
			name:    "extra text before",
			input:   "extra\n-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQD\n-----END PRIVATE KEY-----",
			wantErr: true,
			errMsg:  "TLS key must be in PEM format",
		},
		{
			name:    "extra text after",
			input:   "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQD\n-----END PRIVATE KEY-----\nextra",
			wantErr: true,
			errMsg:  "TLS key must be in PEM format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTlsKey(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("expected error message %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestValidateTlsCa(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty string",
			input:   "",
			wantErr: false,
		},
		{
			name:    "valid PEM CA certificate single line",
			input:   "-----BEGIN CERTIFICATE-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAn\n-----END CERTIFICATE-----",
			wantErr: false,
		},
		{
			name:    "valid PEM CA certificate multiline",
			input:   "-----BEGIN CERTIFICATE-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAn\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAn\n-----END CERTIFICATE-----",
			wantErr: false,
		},
		{
			name:    "missing BEGIN line",
			input:   "MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAn\n-----END CERTIFICATE-----",
			wantErr: true,
			errMsg:  "TLS CA must be in PEM format",
		},
		{
			name:    "missing END line",
			input:   "-----BEGIN CERTIFICATE-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAn",
			wantErr: true,
			errMsg:  "TLS CA must be in PEM format",
		},
		{
			name:    "wrong header",
			input:   "-----BEGIN CERT-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAn\n-----END CERTIFICATE-----",
			wantErr: true,
			errMsg:  "TLS CA must be in PEM format",
		},
		{
			name:    "wrong footer",
			input:   "-----BEGIN CERTIFICATE-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAn\n-----END CERT-----",
			wantErr: true,
			errMsg:  "TLS CA must be in PEM format",
		},
		{
			name:    "no newlines",
			input:   "-----BEGIN CERTIFICATE-----MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAn-----END CERTIFICATE-----",
			wantErr: true,
			errMsg:  "TLS CA must be in PEM format",
		},
		{
			name:    "extra text before",
			input:   "extra\n-----BEGIN CERTIFICATE-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAn\n-----END CERTIFICATE-----",
			wantErr: true,
			errMsg:  "TLS CA must be in PEM format",
		},
		{
			name:    "extra text after",
			input:   "-----BEGIN CERTIFICATE-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAn\n-----END CERTIFICATE-----\nextra",
			wantErr: true,
			errMsg:  "TLS CA must be in PEM format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTlsCa(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("expected error message %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestValidateSreSecretUrl(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "empty string",
			input:   "",
			wantErr: false,
		},
		{
			name:    "simple url",
			input:   "https://secrets.example.com/mysecret",
			wantErr: false,
		},
		{
			name:    "url with path and query",
			input:   "https://secrets.example.com/mysecret?version=1",
			wantErr: false,
		},
		{
			name:    "url with port",
			input:   "http://localhost:8080/secret",
			wantErr: false,
		},
		{
			name:    "non-url string",
			input:   "not a url",
			wantErr: false,
		},
		{
			name:    "random string",
			input:   "abc123!@#",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSreSecretUrl(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestValidateSreCaSecret(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "empty string",
			input:   "",
			wantErr: false,
		},
		{
			name:    "simple string",
			input:   "my-secret",
			wantErr: false,
		},
		{
			name:    "string with special characters",
			input:   "secret@123!#",
			wantErr: false,
		},
		{
			name:    "string with spaces",
			input:   "my secret value",
			wantErr: false,
		},
		{
			name:    "string with unicode",
			input:   "秘密-值",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSreCaSecret(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestValidateSmtpUrl(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "empty string",
			input:   "",
			wantErr: false,
		},
		{
			name:    "simple smtp url",
			input:   "smtp.example.com",
			wantErr: false,
		},
		{
			name:    "smtp url with port",
			input:   "smtp.example.com:587",
			wantErr: false,
		},
		{
			name:    "smtp url with IP address",
			input:   "192.168.1.10:25",
			wantErr: false,
		},
		{
			name:    "smtp url with subdomain",
			input:   "mail.smtp.example.com",
			wantErr: false,
		},
		{
			name:    "smtp url with dash and underscore",
			input:   "smtp-mail_server.example.com",
			wantErr: false,
		},
		{
			name:    "smtp url with user info",
			input:   "user:pass@smtp.example.com:465",
			wantErr: false,
		},
		{
			name:    "smtp url with scheme",
			input:   "smtp://smtp.example.com:25",
			wantErr: false,
		},
		{
			name:    "invalid smtp url with spaces",
			input:   "smtp .example.com",
			wantErr: false,
		},
		{
			name:    "random string",
			input:   "not a url",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSmtpUrl(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestValidateSmtpPort(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty string",
			input:   "",
			wantErr: false,
		},
		{
			name:    "valid port 25",
			input:   "25",
			wantErr: false,
		},
		{
			name:    "valid port 1 (min)",
			input:   "1",
			wantErr: false,
		},
		{
			name:    "valid port 65535 (max)",
			input:   "65535",
			wantErr: false,
		},
		{
			name:    "invalid port 0 (below min)",
			input:   "0",
			wantErr: true,
			errMsg:  "SMTP port must be between 1 and 65535",
		},
		{
			name:    "invalid port 65536 (above max)",
			input:   "65536",
			wantErr: true,
			errMsg:  "SMTP port must be between 1 and 65535",
		},
		{
			name:    "invalid negative port",
			input:   "-25",
			wantErr: true,
			errMsg:  "SMTP port must be between 1 and 65535",
		},
		{
			name:    "invalid non-numeric",
			input:   "abc",
			wantErr: true,
			errMsg:  "cannot convert abc to integer: strconv.Atoi: parsing \"abc\": invalid syntax",
		},
		{
			name:    "invalid float",
			input:   "25.5",
			wantErr: true,
			errMsg:  "cannot convert 25.5 to integer: strconv.Atoi: parsing \"25.5\": invalid syntax",
		},
		{
			name:    "invalid with spaces",
			input:   " 25 ",
			wantErr: true,
			errMsg:  "cannot convert  25  to integer: strconv.Atoi: parsing \" 25 \": invalid syntax",
		},
		{
			name:    "invalid empty space",
			input:   " ",
			wantErr: true,
			errMsg:  "cannot convert   to integer: strconv.Atoi: parsing \" \": invalid syntax",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSmtpPort(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("expected error message %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestValidateSmtpFrom(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "empty string",
			input:   "",
			wantErr: false,
		},
		{
			name:    "simple email",
			input:   "user@example.com",
			wantErr: false,
		},
		{
			name:    "email with dot",
			input:   "first.last@example.com",
			wantErr: false,
		},
		{
			name:    "email with plus",
			input:   "user+test@example.com",
			wantErr: false,
		},
		{
			name:    "email with dash",
			input:   "user-test@example.com",
			wantErr: false,
		},
		{
			name:    "email with underscore",
			input:   "user_test@example.com",
			wantErr: false,
		},
		{
			name:    "invalid email missing @",
			input:   "userexample.com",
			wantErr: false,
		},
		{
			name:    "invalid email missing domain",
			input:   "user@",
			wantErr: false,
		},
		{
			name:    "invalid email missing username",
			input:   "@example.com",
			wantErr: false,
		},
		{
			name:    "random string",
			input:   "not an email",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSmtpFrom(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestValidateIp(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid IP",
			input:   "192.168.1.1",
			wantErr: false,
		},
		{
			name:    "valid IP with zeros",
			input:   "0.0.0.0",
			wantErr: false,
		},
		{
			name:    "valid IP with max octets",
			input:   "255.255.255.255",
			wantErr: false,
		},
		{
			name:    "valid IP with leading zeros",
			input:   "010.001.000.255",
			wantErr: false,
		},
		{
			name:    "invalid IP with too few octets",
			input:   "192.168.1",
			wantErr: true,
			errMsg:  "IP address must follow the format '^([0-9]{1,3}\\.){3}[0-9]{1,3}$', e.g., 192.168.1.1'",
		},
		{
			name:    "invalid IP with too many octets",
			input:   "192.168.1.1.1",
			wantErr: true,
			errMsg:  "IP address must follow the format '^([0-9]{1,3}\\.){3}[0-9]{1,3}$', e.g., 192.168.1.1'",
		},
		{
			name:    "invalid IP with letters",
			input:   "192.abc.1.1",
			wantErr: true,
			errMsg:  "IP address must follow the format '^([0-9]{1,3}\\.){3}[0-9]{1,3}$', e.g., 192.168.1.1'",
		},
		{
			name:    "invalid IP with negative octet",
			input:   "192.168.-1.1",
			wantErr: true,
			errMsg:  "IP address must follow the format '^([0-9]{1,3}\\.){3}[0-9]{1,3}$', e.g., 192.168.1.1'",
		},
		{
			name:    "invalid IP with octet > 255",
			input:   "256.168.1.1",
			wantErr: true,
			errMsg:  "IP address must be between 0 and 255",
		},
		{
			name:    "invalid IP with octet < 0",
			input:   "192.168.1.-2",
			wantErr: true,
			errMsg:  "IP address must follow the format '^([0-9]{1,3}\\.){3}[0-9]{1,3}$', e.g., 192.168.1.1'",
		},
		{
			name:    "invalid IP with special char",
			input:   "192.168.1.a!",
			wantErr: true,
			errMsg:  "IP address must follow the format '^([0-9]{1,3}\\.){3}[0-9]{1,3}$', e.g., 192.168.1.1'",
		},
		{
			name:    "invalid IP with space",
			input:   "192.168. 1.1",
			wantErr: true,
			errMsg:  "IP address must follow the format '^([0-9]{1,3}\\.){3}[0-9]{1,3}$', e.g., 192.168.1.1'",
		},
		{
			name:    "invalid IP with empty string",
			input:   "",
			wantErr: true,
			errMsg:  "IP address must follow the format '^([0-9]{1,3}\\.){3}[0-9]{1,3}$', e.g., 192.168.1.1'",
		},
		{
			name:    "invalid IP with trailing dot",
			input:   "192.168.1.",
			wantErr: true,
			errMsg:  "IP address must follow the format '^([0-9]{1,3}\\.){3}[0-9]{1,3}$', e.g., 192.168.1.1'",
		},
		{
			name:    "invalid IP with leading dot",
			input:   ".192.168.1.1",
			wantErr: true,
			errMsg:  "IP address must follow the format '^([0-9]{1,3}\\.){3}[0-9]{1,3}$', e.g., 192.168.1.1'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateIp(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("expected error message %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestValidateSimpleMode(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "fps only",
			input:   []string{"fps"},
			wantErr: false,
		},
		{
			name:    "fps and ui with eim",
			input:   []string{"fps", "ui", "eim"},
			wantErr: false,
		},
		{
			name:    "fps and ui with ao",
			input:   []string{"fps", "ui", "ao"},
			wantErr: false,
		},
		{
			name:    "fps and ui with co",
			input:   []string{"fps", "ui", "co"},
			wantErr: false,
		},
		{
			name:    "fps and ui with eim and ao",
			input:   []string{"fps", "ui", "eim", "ao"},
			wantErr: false,
		},
		{
			name:    "fps and ui with none of eim, ao, co",
			input:   []string{"fps", "ui"},
			wantErr: true,
			errMsg:  "UI cannot be enabled without at least one of EIM, AO, or CO being enabled",
		},
		{
			name:    "no fps",
			input:   []string{"ui", "eim"},
			wantErr: true,
			errMsg:  "FPS must be enabled",
		},
		{
			name:    "empty slice",
			input:   []string{},
			wantErr: true,
			errMsg:  "FPS must be enabled",
		},
		{
			name:    "fps and unrelated feature",
			input:   []string{"fps", "foo"},
			wantErr: false,
		},
		{
			name:    "fps, ui, unrelated feature",
			input:   []string{"fps", "ui", "foo"},
			wantErr: true,
			errMsg:  "UI cannot be enabled without at least one of EIM, AO, or CO being enabled",
		},
		{
			name:    "fps, ui, eim, unrelated feature",
			input:   []string{"fps", "ui", "eim", "foo"},
			wantErr: false,
		},
		{
			name:    "fps, ui, co, unrelated feature",
			input:   []string{"fps", "ui", "co", "foo"},
			wantErr: false,
		},
		{
			name:    "fps, ui, ao, unrelated feature",
			input:   []string{"fps", "ui", "ao", "foo"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSimpleMode(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("expected error message %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestValidateAdvancedMode(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		wantErr bool
	}{
		{
			name:    "empty slice",
			input:   []string{},
			wantErr: false,
		},
		{
			name:    "single feature",
			input:   []string{"feature1"},
			wantErr: false,
		},
		{
			name:    "multiple features",
			input:   []string{"feature1", "feature2", "feature3"},
			wantErr: false,
		},
		{
			name:    "features with special characters",
			input:   []string{"feat-1", "feat_2", "feat.3"},
			wantErr: false,
		},
		{
			name:    "features with numbers",
			input:   []string{"f1", "f2", "f3"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAdvancedMode(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}
