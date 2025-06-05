// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package config

import "os"

type Scale int

const (
	Scale10    Scale = 10
	Scale100   Scale = 100
	Scale500   Scale = 500
	Scale1000  Scale = 1000
	Scale10000 Scale = 10000
)

// Current version
// Should bump this every time we make backward-compatible config schema changes
const (
	UserConfigVersion   = 4
	RuntimeStateVersion = 2
)

// Minimal version supported by the installer.
// This should never be modified. Create `config/v2` when breaking changes are introduced.
// Files with a version lower than this will require additional migration steps.
const (
	MinUserConfigVersion   = 1
	MinRuntimeStateVersion = 1
)

type OrchInstallerRuntimeState struct {
	Version int `yaml:"version"`
	// The Action that will be performed
	// This can be one of the following:
	// - install
	// - upgrade
	// - uninstall
	Action string `yaml:"action"`

	// The directory where the logs will be saved
	LogDir string `yaml:"logDir"`
	DryRun bool   `yaml:"dryRun"`

	// Targets (Stage or Steps) with any labels matched in this list will be executed (either install, upgrade or uninstall)
	// The installer will execute all targets if this is empty.
	TargetLabels []string `yaml:"targetLabels"`

	// Used for state and o11y bucket prefix. lowercase or digit
	DeploymentID     string `yaml:"deploymentID"`
	StateBucketState string `yaml:"stateBucketState"` // The state S3 bucket Terraform state
	// Move runtime state here?
	KubeConfig               string   `yaml:"kubeConfig"`
	TLSCert                  string   `yaml:"tlsCert"`
	TLSKey                   string   `yaml:"tlsKey"`
	TLSCa                    string   `yaml:"tlsCa"`
	CacheRegistry            string   `yaml:"cacheRegistry"`
	VPCID                    string   `yaml:"vpcID"`
	PublicSubnetIDs          []string `yaml:"publicSubnetIDs"`
	PrivateSubnetIDs         []string `yaml:"privateSubnetIDs"`
	JumpHostSSHKeyPublicKey  string   `yaml:"jumpHostSSHPublicKey"`
	JumpHostSSHKeyPrivateKey string   `yaml:"jumpHostSSHPrivateKey"`
}

type OrchInstallerConfig struct {
	Version  int    `yaml:"version"`
	Provider string `yaml:"provider"`
	Global   struct {
		OrchName     string `yaml:"orchName"`     // EMF deployment name
		ParentDomain string `yaml:"parentDomain"` // not including cluster name
		AdminEmail   string `yaml:"adminEmail"`
		Scale        Scale  `yaml:"scale"`
	} `yaml:"global"`
	Advanced struct { // TODO: form for this part is not done yet
		AzureADRefreshToken  string `yaml:"azureADRefreshToken,omitempty"`
		AzureADTokenEndpoint string `yaml:"azureADTokenEndpoint,omitempty"`
	} `yaml:"advanced"`
	AWS struct {
		Region                string   `yaml:"region"`
		CustomerTag           string   `yaml:"customerTag,omitempty"`
		CacheRegistry         string   `yaml:"cacheRegistry,omitempty"`
		JumpHostWhitelist     []string `yaml:"jumpHostWhitelist,omitempty"`
		JumpHostIP            string   `yaml:"jumpHostIP,omitempty"`
		JumpHostPrivKeyPath   string   `yaml:"jumpHostPrivKeyPath,omitempty"`
		VPCID                 string   `yaml:"vpcID,omitempty"`
		ReduceNSTTL           bool     `yaml:"reduceNSTTL,omitempty"` // TODO: do we need this?
		EKSDNSIP              string   `yaml:"eksDNSIP,omitempty"`    // TODO: do we need this?
		EKSIAMRoles           []string `yaml:"eksIAMRoles,omitempty"`
		PreviousS3StateBucket string   `yaml:"previousS3StateBucket,omitempty"` // The S3 bucket where the previous state is stored, will be deprecated in version 3.2.
	} `yaml:"aws,omitempty"`
	Onprem struct {
		ArgoIP         string `yaml:"argoIP"`
		TraefikIP      string `yaml:"traefikIP"`
		NginxIP        string `yaml:"nginxIP"`
		DockerUsername string `yaml:"dockerUsername,omitempty"`
		DockerToken    string `yaml:"dockerToken,omitempty"`
	} `yaml:"onprem,omitempty"`
	Orch struct {
		Enabled         []string `yaml:"enabled"`
		DefaultPassword string   `yaml:"defaultPassword"`
	} `yaml:"orch"`
	// Optional
	Cert struct {
		TLSCert string `yaml:"tlsCert,omitempty"`
		TLSKey  string `yaml:"tlsKey,omitempty"`
		TLSCA   string `yaml:"tlsCA,omitempty"`
	} `yaml:"cert,omitempty"`
	SRE struct {
		Username  string `yaml:"username,omitempty"`
		Password  string `yaml:"password,omitempty"`
		SecretUrl string `yaml:"secretURL,omitempty"`
		CASecret  string `yaml:"caSecret,omitempty"`
	} `yaml:"sre,omitempty"`
	SMTP struct {
		Username string `yaml:"username"`
		Password string `yaml:"password"`
		URL      string `yaml:"url"`
		Port     string `yaml:"port"`
		From     string `yaml:"from"`
	} `yaml:"smtp,omitempty"`
	Proxy struct {
		HTTPProxy    string `yaml:"httpProxy,omitempty"`
		HTTPSProxy   string `yaml:"httpsProxy,omitempty"`
		SocksProxy   string `yaml:"socksProxy,omitempty"`
		NoProxy      string `yaml:"noProxy,omitempty"`
		ENHTTPProxy  string `yaml:"enHttpProxy,omitempty"`
		ENHTTPSProxy string `yaml:"enHttpsProxy,omitempty"`
		ENFTPProxy   string `yaml:"enFtpProxy,omitempty"`
		ENSocksProxy string `yaml:"enSocksProxy,omitempty"`
		ENNoProxy    string `yaml:"enNoProxy,omitempty"`
	} `yaml:"proxy,omitempty"`
}

type OrchApp struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

type OrchPackage struct {
	Name        string             `yaml:"name"`
	Description string             `yaml:"description"`
	Apps        map[string]OrchApp `yaml:"apps"`
}

type OrchConfigReaderWriter interface {
	WriteOrchConfig(orchConfig OrchInstallerConfig) error
	ReadOrchConfig() (OrchInstallerConfig, error)
	WriteRuntimeState(runtimeState OrchInstallerRuntimeState) error
	ReadRuntimeState() (OrchInstallerRuntimeState, error)
}

type FileBaseOrchConfigReaderWriter struct {
	OrchConfigFilePath   string
	RuntimeStateFilePath string
}

func (f *FileBaseOrchConfigReaderWriter) WriteOrchConfig(orchConfig OrchInstallerConfig) error {
	orchConfigYaml, err := SerializeToYAML(orchConfig)
	if err != nil {
		return err
	}
	return os.WriteFile(f.OrchConfigFilePath, orchConfigYaml, 0o644)
}

func (f *FileBaseOrchConfigReaderWriter) ReadOrchConfig() (OrchInstallerConfig, error) {
	orchConfig := OrchInstallerConfig{}
	orchConfigData, err := os.ReadFile(f.OrchConfigFilePath)
	if err != nil {
		return orchConfig, err
	}
	err = DeserializeFromYAML(&orchConfig, orchConfigData)
	if err != nil {
		return orchConfig, err
	}
	return orchConfig, nil
}

func (f *FileBaseOrchConfigReaderWriter) WriteRuntimeState(runtimeState OrchInstallerRuntimeState) error {
	runtimeStateYaml, err := SerializeToYAML(runtimeState)
	if err != nil {
		return err
	}
	return os.WriteFile(f.RuntimeStateFilePath, runtimeStateYaml, 0o644)
}

func (f *FileBaseOrchConfigReaderWriter) ReadRuntimeState() (OrchInstallerRuntimeState, error) {
	runtimeState := OrchInstallerRuntimeState{}
	runtimeStateData, err := os.ReadFile(f.RuntimeStateFilePath)
	if err != nil {
		return runtimeState, err
	}
	err = DeserializeFromYAML(&runtimeState, runtimeStateData)
	if err != nil {
		return runtimeState, err
	}
	return runtimeState, nil
}
