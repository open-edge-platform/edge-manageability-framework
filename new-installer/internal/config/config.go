// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package config

type Scale int

const (
	Scale10    Scale = 10
	Scale100   Scale = 100
	Scale500   Scale = 500
	Scale1000  Scale = 1000
	Scale10000 Scale = 10000
)

type OrchInstallerRuntimeState struct {
	// The Action that will be performed
	// This can be one of the following:
	// - install
	// - upgrade
	// - uninstall
	Action string `yaml:"action"`

	// The directory where the logs will be saved
	LogDir string `yaml:"logDir"`
	DryRun bool   `yaml:"dryRun"`

	// Used for state and o11y bucket prefix. lowercase or digit
	DeploymentId     string `yaml:"deploymentId"`
	StateBucketState string `yaml:"stateBucketState"` // The state S3 bucket Terraform state
	// Move runtime state here?
	KubeConfig    string `yaml:"kubeConfig"`
	TlsCert       string `yaml:"tlsCert"`
	TlsKey        string `yaml:"tlsKey"`
	TlsCa         string `yaml:"tlsCa"`
	CacheRegistry string `yaml:"cacheRegistry"`
	VpcId         string `yaml:"vpcId"` // VPC ID

	PublicSubnetIds          []string `yaml:"publicSubnetIds"`
	PrivateSubnetIds         []string `yaml:"privateSubnetIds"`
	JumpHostSSHKeyPublicKey  string   `yaml:"jumpHostSshPublicKey"`
	JumpHostSSHKeyPrivateKey string   `yaml:"jumpHostSshPrivateKey"`
}

type OrchInstallerConfig struct {
	Version   int                       `yaml:"version"`
	Provider  string                    `yaml:"provider"`
	Generated OrchInstallerRuntimeState `yaml:"generated"`
	Global    struct {
		OrchName     string `yaml:"orchName"`     // EMF deployment name
		ParentDomain string `yaml:"parentDomain"` // not including cluster name
		AdminEmail   string `yaml:"adminEmail"`
		Scale        Scale  `yaml:"scale"`
	} `yaml:"global"`
	Advanced struct { // TODO: form for this part is not done yet
		Enabled              []string `yaml:"enabled"` // installer module flag
		AzureAdRefreshToken  string   `yaml:"azureAdRefreshToken,omitempty"`
		AzureAdTokenEndpoint string   `yaml:"azureAdTokenEndpoint,omitempty"`
	} `yaml:"advanced"`
	Aws struct {
		Region            string   `yaml:"region"`
		CustomerTag       string   `yaml:"customerTag,omitempty"`
		CacheRegistry     string   `yaml:"cacheRegistry,omitempty"`
		JumpHostWhitelist []string `yaml:"jumpHostWhitelist,omitempty"`
		VpcId             string   `yaml:"vpcId,omitempty"`
		ReduceNsTtl       bool     `yaml:"reduceNsTtl,omitempty"` // TODO: do we need this?
		EksDnsIp          string   `yaml:"eksDnsIp,omitempty"`    // TODO: do we need this?
	} `yaml:"aws,omitempty"`
	Onprem struct {
		ArgoIP         string `yaml:"argoIp"`
		TraefikIP      string `yaml:"traefikIp"`
		NginxIP        string `yaml:"nginxIp"`
		DockerUsername string `yaml:"dockerUsername,omitempty"`
		DockerToken    string `yaml:"dockerToken,omitempty"`
	} `yaml:"onprem,omitempty"`
	Orch struct {
		Enabled         []string `yaml:"enabled"`
		DefaultPassword string   `yaml:"defaultPassword"`
	} `yaml:"orch"`
	// Optional
	Cert struct {
		TlsCert string `yaml:"tlsCert,omitempty"`
		TlsKey  string `yaml:"tlsKey,omitempty"`
		TlsCa   string `yaml:"tlsCa,omitempty"`
	} `yaml:"cert,omitempty"`
	Sre struct {
		Username  string `yaml:"username,omitempty"`
		Password  string `yaml:"password,omitempty"`
		SecretUrl string `yaml:"secretUrl,omitempty"`
		CaSecret  string `yaml:"caSecret,omitempty"`
	} `yaml:"sre,omitempty"`
	Smtp struct {
		Username string `yaml:"username"`
		Password string `yaml:"password"`
		Url      string `yaml:"url"`
		Port     string `yaml:"port"`
		From     string `yaml:"from"`
	} `yaml:"smtp,omitempty"`
	Proxy struct {
		HttpProxy  string `yaml:"httpProxy,omitempty"`
		HttpsProxy string `yaml:"httpsProxy,omitempty"`
		SocksProxy string `yaml:"socksProxy,omitempty"`
		NoProxy    string `yaml:"noProxy,omitempty"`
	} `yaml:"proxy,omitempty"`
}

type orchApp struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

type orchPackage struct {
	Name        string             `yaml:"name"`
	Description string             `yaml:"description"`
	Apps        map[string]orchApp `yaml:"apps"`
}
