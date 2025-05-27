// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package steps_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/knadh/koanf/v2"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
	"github.com/stretchr/testify/suite"
)

type TerraformUtilityTest struct {
	suite.Suite
	testdataDir string
	tfExecPath  string
}

func TestTerraformUtility(t *testing.T) {
	suite.Run(t, new(TerraformUtilityTest))
}

func (s *TerraformUtilityTest) SetupTest() {
	var err error
	s.tfExecPath, err = steps.InstallTerraformAndGetExecPath()
	if !s.NoError(err) {
		return
	}
	s.testdataDir, err = filepath.Abs("./testdata")
	if !s.NoError(err) {
		return
	}
	s.deleteTerraformFiles()
}

func (s *TerraformUtilityTest) TearDownTest() {
	s.deleteTerraformFiles()
}

type TestTfVariables struct {
	Var1 string `json:"var1"`
	Var2 int    `json:"var2"`
}

func (s *TerraformUtilityTest) TestApplyingTerraformModule() {
	tfUtil := steps.CreateTerraformUtility()
	ctx := context.Background()
	variables := TestTfVariables{
		Var1: "value1",
		Var2: 2,
	}
	output, err := tfUtil.Run(ctx, steps.TerraformUtilityInput{
		Action:             "install",
		ExecPath:           s.tfExecPath,
		ModulePath:         s.testdataDir,
		LogFile:            filepath.Join(s.testdataDir, "terraform.log"),
		KeepGeneratedFiles: false,
		Variables:          variables,
	})
	if err != nil {
		s.NoError(err)
		return
	}
	output1, uerr := output.Output["output1"].Value.MarshalJSON()
	if !s.NoError(uerr) {
		return
	}
	s.Equal(output1, []byte(`"value1"`))
	output2, uerr := output.Output["output2"].Value.MarshalJSON()
	if !s.NoError(uerr) {
		return
	}
	s.Equal(output2, []byte(`2`))

	s.NotEmpty(output.TerraformState)
	k := koanf.New(".")
	k.Load(rawbytes.Provider([]byte(output.TerraformState)), json.Parser())
	res := k.Get("resources").([]any)
	s.NotEmpty(res)
}

func (s *TerraformUtilityTest) TestDestroyTerraformModule() {
	tfUtil := steps.CreateTerraformUtility()
	ctx := context.Background()
	variables := TestTfVariables{
		Var1: "value1",
		Var2: 2,
	}
	tfState, err := os.ReadFile(filepath.Join(s.testdataDir, "teststate.json"))
	if !s.NoError(err) {
		return
	}
	output, utilErr := tfUtil.Run(ctx, steps.TerraformUtilityInput{
		Action:             "uninstall",
		ExecPath:           s.tfExecPath,
		ModulePath:         s.testdataDir,
		LogFile:            filepath.Join(s.testdataDir, "terraform.log"),
		KeepGeneratedFiles: false,
		Variables:          variables,
		TerraformState:     string(tfState),
	})
	if utilErr != nil {
		s.NoError(utilErr)
		return
	}
	s.Empty(output.Output)

	s.NotEmpty(output.TerraformState)
	k := koanf.New(".")
	k.Load(rawbytes.Provider([]byte(output.TerraformState)), json.Parser())
	res := k.Get("resources").([]any)
	s.Empty(res)
}

func (s *TerraformUtilityTest) deleteTerraformFiles() {
	entries, err := os.ReadDir(s.testdataDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		path := filepath.Join(s.testdataDir, entry.Name())
		if entry.IsDir() {
			if entry.Name() == ".terraform" || entry.Name() == "environments" {
				_ = os.RemoveAll(path)
			}
		} else {
			ext := filepath.Ext(entry.Name())
			if ext == ".tfstate" || ext == ".backup" || ext == ".log" {
				_ = os.Remove(path)
			}
			if entry.Name() == ".terraform.lock.hcl" {
				_ = os.Remove(path)
			}
		}
	}
}
