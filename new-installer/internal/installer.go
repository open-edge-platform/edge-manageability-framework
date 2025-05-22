// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"fmt"
	"sync"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
)

type OrchInstaller struct {
	Stages []OrchInstallerStage

	mutex     *sync.Mutex
	cancelled bool
}

type Scale int

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
		HttpProxy    string `yaml:"httpProxy,omitempty"`
		HttpsProxy   string `yaml:"httpsProxy,omitempty"`
		SocksProxy   string `yaml:"socksProxy,omitempty"`
		NoProxy      string `yaml:"noProxy,omitempty"`
		AdminEmail   string `yaml:"adminEmail"`
		Scale        Scale  `yaml:"scale"`
	} `yaml:"global"`
	Advanced struct {
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
		IP             string `yaml:"ip"`
		DockerUsername string `yaml:"dockerUsername,omitempty"`
		DockerToken    string `yaml:"dockerToken,omitempty"`
	} `yaml:"onprem,omitempty"`
	Orch struct {
		Enabled         []string `yaml:"enabled"`
		DefaultPassword string   `yaml:"defaultPassword"`
	} `yaml:"orch"`
	// Optional
	Cert struct {
		TlsCert string `yaml:"tlsCert"`
		TlsKey  string `yaml:"tlsKey"`
		TlsCa   string `yaml:"tlsCa"`
	} `yaml:"cert,omitempty"`
	Sre struct {
		username  string `yaml:"username"`
		password  string `yaml:"password"`
		secretUrl string `yaml:"secretUrl"`
		caSecret  string `yaml:"caSecret"`
	} `yaml:"sre,omitempty"`
	Smtp struct {
		username string `yaml:"username"`
		password string `yaml:"password"`
		url      string `yaml:"url"`
		port     int    `yaml:"port"`
		from     string `yaml:"from"`
	} `yaml:"smtp,omitempty"`
}

func UpdateRuntimeState(dest *OrchInstallerRuntimeState, source OrchInstallerRuntimeState) *OrchInstallerError {
	srcK := koanf.New(".")
	srcK.Load(structs.Provider(source, "yaml"), nil)
	dstK := koanf.New(".")
	dstK.Load(structs.Provider(dest, "yaml"), nil)
	dstK.Merge(srcK)

	dstData, err := dstK.Marshal(yaml.Parser())
	if err != nil {
		return &OrchInstallerError{
			ErrorCode: OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to marshal runtime state: %v", err),
		}
	}

	err = DeserializeFromYAML(dest, dstData)
	if err != nil {
		return &OrchInstallerError{
			ErrorCode: OrchInstallerErrorCodeInternal,
			ErrorMsg:  fmt.Sprintf("failed to unmarshal runtime state: %v", err),
		}
	}
	return nil
}

func CreateOrchInstaller(stages []OrchInstallerStage) (*OrchInstaller, error) {
	return &OrchInstaller{
		Stages:    stages,
		mutex:     &sync.Mutex{},
		cancelled: false,
	}, nil
}

func reverseStages(stages []OrchInstallerStage) []OrchInstallerStage {
	reversed := []OrchInstallerStage{}
	for i := len(stages) - 1; i >= 0; i-- {
		reversed = append(reversed, stages[i])
	}
	return reversed
}

func (o *OrchInstaller) Run(ctx context.Context, config OrchInstallerConfig) *OrchInstallerError {
	logger := Logger()
	action := config.Generated.Action
	if action == "" {
		return &OrchInstallerError{
			ErrorCode: OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  "action must be specified",
		}
	}

	if action != "install" && action != "upgrade" && action != "uninstall" {
		return &OrchInstallerError{
			ErrorCode: OrchInstallerErrorCodeInvalidArgument,
			ErrorMsg:  fmt.Sprintf("unsupported action: %s", action),
		}
	}
	if action == "uninstall" {
		o.Stages = reverseStages(o.Stages)
	}
	for _, stage := range o.Stages {
		var err *OrchInstallerStageError
		if o.Cancelled() {
			logger.Info("Installation cancelled")
			break
		}
		name := stage.Name()
		logger.Infof("Running stage: %s", name)
		err = stage.PreStage(ctx, &config)

		// We will skip to run the stage if the previous stage failed
		if err == nil {
			err = stage.RunStage(ctx, &config)
		}

		// But we will always run the post stage, the post stage should
		// handle the error and rollback if needed.
		err = stage.PostStage(ctx, &config, err)
		if err != nil {
			return &OrchInstallerError{
				ErrorCode: OrchInstallerErrorCodeInternal,
				ErrorMsg:  BuildErrorMessage(name, err),
			}
		}
	}
	return nil
}

func (o *OrchInstaller) CancelInstallation() {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	o.cancelled = true
}

func (o *OrchInstaller) Cancelled() bool {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	return o.cancelled
}

func BuildErrorMessage(stageName string, err *OrchInstallerStageError) string {
	if err == nil {
		return ""
	}
	msg := "Stage: " + stageName + "\n"
	for name, stepErr := range err.StepErrors {
		if stepErr != nil {
			msg += fmt.Sprintf("Step: %s\n", name)
			msg += fmt.Sprintf("Error: %s\n", stepErr.ErrorMsg)
		}
	}
	return msg
}
