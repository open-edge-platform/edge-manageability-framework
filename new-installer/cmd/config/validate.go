// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

func validateOrchName(s string) error {
	if len(s) == 0 {
		return fmt.Errorf("orchestrator name cannot be empty")
	}
	if len(s) >= 16 {
		return fmt.Errorf("orchestrator name must be less than 16 characters")
	}
	if matched := regexp.MustCompile(`^[a-z0-9]+$`).MatchString(s); !matched {
		return fmt.Errorf("orchestrator name must be all lower case letters or digits")
	}
	return nil
}

func validateParentDomain(s string) error {
	if matched := regexp.MustCompile(`^[a-z0-9.-]+$`).MatchString(s); !matched {
		return fmt.Errorf("parent domain must be all lower case letters, digits, or '.'")
	}
	return nil
}

func validateAdminEmail(s string) error {
	if matched := regexp.MustCompile(`^[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}$`).MatchString(s); !matched {
		return fmt.Errorf("admin email must be a valid email address")
	}
	return nil
}

func validateAwsRegion(s string) error {
	if matched := regexp.MustCompile(`^[a-z]+-[a-z]+-\d$`).MatchString(s); !matched {
		return fmt.Errorf("region must follow the format '^[a-z]+-[a-z]+-\\d$', e.g., 'us-west-2'")
	}
	return nil
}

func validateAwsCustomTag(s string) error {
	return nil
}

func validateCacheRegistry(s string) error {
	return nil
}

func validateAwsJumpHostWhitelist(s string) error {
	return nil
}

func validateAwsVpcId(s string) error {
	if s == "" {
		return nil
	}
	if matched := regexp.MustCompile(`^vpc-[0-9a-f]{8}$`).MatchString(s); !matched {
		return fmt.Errorf("VPC ID must follow the format '^vpc-[0-9a-f]{8}$', e.g., 'vpc-12345678'")
	}
	return nil
}

func validateAwsEksDnsIp(s string) error {
	if s == "" {
		return nil
	}
	if matched := regexp.MustCompile(`^([0-9]{1,3}\.){3}[0-9]{1,3}$`).MatchString(s); !matched {
		return fmt.Errorf("EKS DNS IP must follow the format '^([0-9]{1,3}\\.){3}[0-9]{1,3}$', e.g., '")
	}
	return nil
}

func validateProxy(s string) error {
	// TODO: implement proxy validation
	return nil
}

func validateTlsCert(s string) error {
	if s == "" {
		return nil
	}
	if matched := regexp.MustCompile(`^-----BEGIN CERTIFICATE-----\n.*\n-----END CERTIFICATE-----$`).MatchString(s); !matched {
		return fmt.Errorf("TLS certificate must be in PEM format")
	}
	return nil
}

func validateTlsKey(s string) error {
	if s == "" {
		return nil
	}
	if matched := regexp.MustCompile(`^-----BEGIN PRIVATE KEY-----\n.*\n-----END PRIVATE KEY-----$`).MatchString(s); !matched {
		return fmt.Errorf("TLS key must be in PEM format")
	}
	return nil
}

func validateTlsCa(s string) error {
	if s == "" {
		return nil
	}
	if matched := regexp.MustCompile(`^-----BEGIN CERTIFICATE-----\n.*\n-----END CERTIFICATE-----$`).MatchString(s); !matched {
		return fmt.Errorf("TLS CA must be in PEM format")
	}
	return nil
}

func validateSreSecretUrl(s string) error {
	return nil
}

func validateSreCaSecret(s string) error {
	return nil
}

func validateSmtpUrl(s string) error {
	return nil
}

func validateSmtpPort(s string) error {
	if s == "" {
		return nil
	}
	i, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("cannot convert %s to integer: %s", s, err)
	}
	if i < 1 || i > 65535 {
		return fmt.Errorf("SMTP port must be between 1 and 65535")
	}
	return nil
}

func validateSmtpFrom(s string) error {
	return nil
}

func validateIp(s string) error {
	if matched := regexp.MustCompile(`^([0-9]{1,3}\.){3}[0-9]{1,3}$`).MatchString(s); !matched {
		return fmt.Errorf("IP address must follow the format '^([0-9]{1,3}\\.){3}[0-9]{1,3}$', e.g., 192.168.1.1'")
	}
	parts := strings.Split(s, ".")
	for _, part := range parts {
		i, err := strconv.Atoi(part)
		if err != nil {
			return fmt.Errorf("cannot convert %s to integer: %s", part, err)
		}
		if i < 0 || i > 255 {
			return fmt.Errorf("IP address must be between 0 and 255")
		}
	}
	return nil
}

func validateSimpleMode(s []string) error {
	if !slices.Contains(s, "fps") {
		return fmt.Errorf("FPS must be enabled")
	}
	if slices.Contains(s, "ui") &&
		!slices.Contains(s, "eim") &&
		!slices.Contains(s, "co") &&
		!slices.Contains(s, "ao") {
		return fmt.Errorf("UI cannot be enabled without at least one of EIM, AO, or CO being enabled")
	}
	return nil
}

func validateAdvancedMode(s []string) error {
	// TODO: placeholder for advanced mode validation
	return nil
}
