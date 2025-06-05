// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
)

func validateAll() error {
	if err := validateOrchName(input.Global.OrchName); err != nil {
		return fmt.Errorf("invalid orchestrator name: %w", err)
	}
	if err := validateParentDomain(input.Global.ParentDomain); err != nil {
		return fmt.Errorf("invalid parent domain: %w", err)
	}
	if err := validateAdminEmail(input.Global.AdminEmail); err != nil {
		return fmt.Errorf("invalid admin email: %w", err)
	}
	if err := validateScale(input.Global.Scale); err != nil {
		return fmt.Errorf("invalid scale: %w", err)
	}
	if err := validateAwsRegion(input.AWS.Region); err != nil {
		return fmt.Errorf("invalid AWS region: %w", err)
	}
	if err := validateAwsCustomTag(input.AWS.CustomerTag); err != nil {
		return fmt.Errorf("invalid AWS custom tag: %w", err)
	}
	if err := validateCacheRegistry(input.AWS.CacheRegistry); err != nil {
		return fmt.Errorf("invalid cache registry: %w", err)
	}
	if err := validateAwsJumpHostWhitelist(config.SliceToCommaSeparated(input.AWS.JumpHostWhitelist)); err != nil {
		return fmt.Errorf("invalid AWS jump host whitelist: %w", err)
	}
	if err := validateOptionalIP(input.AWS.JumpHostIP); err != nil {
		return fmt.Errorf("invalid AWS jump host IP: %w", err)
	}
	if err := validateJumpHostPrivKeyPath(input.AWS.JumpHostPrivKeyPath); err != nil {
		return fmt.Errorf("invalid AWS jump host private key path: %w", err)
	}
	if err := validateAwsVpcId(input.AWS.VPCID); err != nil {
		return fmt.Errorf("invalid AWS VPC ID: %w", err)
	}
	if err := validateAwsEksDnsIp(input.AWS.EKSDNSIP); err != nil {
		return fmt.Errorf("invalid AWS EKS DNS IP: %w", err)
	}
	if err := validateAwsEKSIAMRoles(config.SliceToCommaSeparated(input.AWS.EKSIAMRoles)); err != nil {
		return fmt.Errorf("invalid AWS EKS IAM roles: %w", err)
	}
	if err := validateProxy(input.Proxy.HTTPProxy); err != nil {
		return fmt.Errorf("invalid HTTP proxy: %w", err)
	}
	if err := validateProxy(input.Proxy.HTTPSProxy); err != nil {
		return fmt.Errorf("invalid HTTPS proxy: %w", err)
	}
	if err := validateProxy(input.Proxy.SOCKSProxy); err != nil {
		return fmt.Errorf("invalid SOCKS proxy: %w", err)
	}
	if err := validateNoProxy(input.Proxy.NoProxy); err != nil {
		return fmt.Errorf("invalid no proxy: %w", err)
	}
	if err := validateProxy(input.Proxy.ENHTTPProxy); err != nil {
		return fmt.Errorf("invalid EN HTTP proxy: %w", err)
	}
	if err := validateProxy(input.Proxy.ENHTTPSProxy); err != nil {
		return fmt.Errorf("invalid EN HTTPS proxy: %w", err)
	}
	if err := validateProxy(input.Proxy.ENFTPProxy); err != nil {
		return fmt.Errorf("invalid EN FTP proxy: %w", err)
	}
	if err := validateProxy(input.Proxy.ENSOCKSProxy); err != nil {
		return fmt.Errorf("invalid EN SOCKS proxy: %w", err)
	}
	if err := validateNoProxy(input.Proxy.ENNoProxy); err != nil {
		return fmt.Errorf("invalid ENno proxy: %w", err)
	}
	if err := validateTlsCert(input.Cert.TLSCert); err != nil {
		return fmt.Errorf("invalid TLS certificate: %w", err)
	}
	if err := validateTlsKey(input.Cert.TLSKey); err != nil {
		return fmt.Errorf("invalid TLS key: %w", err)
	}
	if err := validateTlsCa(input.Cert.TLSCA); err != nil {
		return fmt.Errorf("invalid TLS CA: %w", err)
	}
	if err := validateSreSecretUrl(input.SRE.SecretUrl); err != nil {
		return fmt.Errorf("invalid SRE secret URL: %w", err)
	}
	if err := validateSreCaSecret(input.SRE.CASecret); err != nil {
		return fmt.Errorf("invalid SRE CA secret: %w", err)
	}
	if err := validateSmtpUrl(input.SMTP.URL); err != nil {
		return fmt.Errorf("invalid SMTP URL: %w", err)
	}
	if err := validateSmtpPort(input.SMTP.Port); err != nil {
		return fmt.Errorf("invalid SMTP port: %w", err)
	}
	if err := validateSmtpFrom(input.SMTP.From); err != nil {
		return fmt.Errorf("invalid SMTP from address: %w", err)
	}
	if err := validateAdvancedMode(input.Orch.Enabled); err != nil {
		return fmt.Errorf("invalid advanced mode configuration: %w", err)
	}
	return nil
}

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
	if matched := regexp.MustCompile(`^[a-z0-9-.]+\.[a-z0-9-]+$`).MatchString(s); !matched {
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

func validateScale(s config.Scale) error {
	if !s.IsValid() {
		return fmt.Errorf("scale must be one of: %v", config.ValidScales())
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
	if s == "" {
		return nil
	}
	re := regexp.MustCompile(`^https?://[a-z0-9.-]+(:\d+)?$`)
	if !re.MatchString(s) {
		return fmt.Errorf("proxy must be in the format http(s)://host[:port], e.g., http://proxy.intel.com:912")
	}
	return nil
}

func validateNoProxy(s string) error {
	if s == "" {
		return nil
	}
	entries := strings.Split(s, ",")
	ipRe := regexp.MustCompile(`^([0-9]{1,3}\.){3}[0-9]{1,3}(\/\d{1,2})?$`)
	domainRe := regexp.MustCompile(`^\.?([a-z0-9.-]+\.[a-z]{2,})$`)
	for _, entry := range entries {
		e := strings.TrimSpace(entry)
		if e == "" {
			continue
		}
		if ipRe.MatchString(e) {
			// Validate IP/CIDR
			ip := e
			if idx := strings.Index(e, "/"); idx != -1 {
				ip = e[:idx]
				mask := e[idx+1:]
				m, err := strconv.Atoi(mask)
				if err != nil || m < 0 || m > 32 {
					return fmt.Errorf("invalid CIDR mask in no_proxy entry: %s", e)
				}
			}
			parts := strings.Split(ip, ".")
			for _, part := range parts {
				i, err := strconv.Atoi(part)
				if err != nil || i < 0 || i > 255 {
					return fmt.Errorf("invalid IP in no_proxy entry: %s", e)
				}
			}
			continue
		}
		if domainRe.MatchString(e) {
			continue
		}
		return fmt.Errorf("invalid no_proxy entry: %s", e)
	}
	return nil
}

func validateTlsCert(s string) error {
	if s == "" {
		return nil
	}
	if matched := regexp.MustCompile(`(?s)^-----BEGIN CERTIFICATE-----\n.*\n-----END CERTIFICATE-----\n?$`).MatchString(s); !matched {
		return fmt.Errorf("TLS certificate must be in PEM format")
	}
	return nil
}

func validateTlsKey(s string) error {
	if s == "" {
		return nil
	}
	if matched := regexp.MustCompile(`(?s)^-----BEGIN PRIVATE KEY-----\n.*\n-----END PRIVATE KEY-----\n?$`).MatchString(s); !matched {
		return fmt.Errorf("TLS key must be in PEM format")
	}
	return nil
}

func validateTlsCa(s string) error {
	if s == "" {
		return nil
	}
	if matched := regexp.MustCompile(`(?s)^-----BEGIN CERTIFICATE-----\n.*\n-----END CERTIFICATE-----\n?$`).MatchString(s); !matched {
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
		return fmt.Errorf("cannot convert %s to integer: %w", s, err)
	}
	if i < 1 || i > 65535 {
		return fmt.Errorf("SMTP port must be between 1 and 65535")
	}
	return nil
}

func validateSmtpFrom(s string) error {
	return nil
}

func validateIP(s string) error {
	return validateIPInternal(s, false)
}

func validateOptionalIP(s string) error {
	return validateIPInternal(s, true)
}

func validateIPInternal(s string, allowEmpty bool) error {
	if s == "" && allowEmpty {
		return nil
	}
	if matched := regexp.MustCompile(`^([0-9]{1,3}\.){3}[0-9]{1,3}$`).MatchString(s); !matched {
		return fmt.Errorf("IP address must follow the format '^([0-9]{1,3}\\.){3}[0-9]{1,3}$', e.g., 192.168.1.1'")
	}
	parts := strings.Split(s, ".")
	for _, part := range parts {
		i, _ := strconv.Atoi(part)
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

func validateJumpHostPrivKeyPath(s string) error {
	if s == "" {
		return nil
	}
	s = os.ExpandEnv(s)
	if _, err := os.Stat(s); err != nil {
		return fmt.Errorf("jump host private key file does not exist: %w", err)
	}
	// TODO: check if the file content is a valid private key
	return nil
}

func validateAwsEKSIAMRoles(s string) error {
	if s == "" {
		return nil
	}
	roles := strings.Split(s, ",")
	for _, role := range roles {
		role = strings.TrimSpace(role)
		if role == "" {
			continue
		}
		if matched := regexp.MustCompile(`^arn:aws:iam::\d{12}:role/[\w+=,.@-]+$`).MatchString(role); !matched {
			return fmt.Errorf("invalid IAM role ARN: %s", role)
		}
	}
	return nil
}
