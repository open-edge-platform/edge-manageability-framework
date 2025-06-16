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
	s.Require().NoError(err)
	s.testdataDir, err = filepath.Abs("./testdata")
	s.Require().NoError(err)
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
	tfUtil, err := steps.CreateTerraformUtility(s.tfExecPath)
	if err != nil {
		s.NoError(err)
		return
	}
	ctx := context.Background()
	variables := TestTfVariables{
		Var1: "value1",
		Var2: 2,
	}
	output, err := tfUtil.Run(ctx, steps.TerraformUtilityInput{
		Action:             "install",
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
	s.Require().NoError(uerr)
	s.Equal(output1, []byte(`"value1"`))
	output2, uerr := output.Output["output2"].Value.MarshalJSON()
	s.Require().NoError(uerr)
	s.Equal(output2, []byte(`2`))

	s.NotEmpty(output.TerraformState)
	k := koanf.New(".")
	loadErr := k.Load(rawbytes.Provider([]byte(output.TerraformState)), json.Parser())
	s.Require().NoError(loadErr)
	res := k.Get("resources").([]any)
	s.NotEmpty(res)
}

func (s *TerraformUtilityTest) TestDestroyTerraformModule() {
	tfUtil, initErr := steps.CreateTerraformUtility(s.tfExecPath)
	if initErr != nil {
		s.NoError(initErr)
		return
	}
	ctx := context.Background()
	variables := TestTfVariables{
		Var1: "value1",
		Var2: 2,
	}
	tfState, err := os.ReadFile(filepath.Join(s.testdataDir, "teststate.json"))
	s.Require().NoError(err)
	output, utilErr := tfUtil.Run(ctx, steps.TerraformUtilityInput{
		Action:             "uninstall",
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
	loadErr := k.Load(rawbytes.Provider([]byte(output.TerraformState)), json.Parser())
	s.Require().NoError(loadErr)
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

func (s *TerraformUtilityTest) TestDestroyResource() {
	tfUtil, initErr := steps.CreateTerraformUtility(s.tfExecPath)
	if initErr != nil {
		s.NoError(initErr)
		return
	}
	ctx := context.Background()
	tfState, err := os.ReadFile(filepath.Join(s.testdataDir, "teststate.json"))
	s.Require().NoError(err)
	tfStateJson, err := json.Parser().Unmarshal(tfState)
	s.Require().NoError(err, "Expected to unmarshal the Terraform state JSON")
	s.NotNil(tfStateJson["resources"], "Expected resources in the state file")
	s.Len(tfStateJson["resources"].([]any), 2, "Expected two resources in the state file")
	s.Equal("res1", tfStateJson["resources"].([]interface{})[0].(map[string]interface{})["name"], "Expected first resource name to be res1")
	s.Equal("res2", tfStateJson["resources"].([]interface{})[1].(map[string]interface{})["name"], "Expected first resource name to be res2")

	variables := TestTfVariables{
		Var1: "value1",
		Var2: 2,
	}

	output, utilErr := tfUtil.Run(ctx, steps.TerraformUtilityInput{
		Action:             "uninstall",
		ModulePath:         s.testdataDir,
		LogFile:            filepath.Join(s.testdataDir, "terraform.log"),
		KeepGeneratedFiles: false,
		Variables:          variables,
		TerraformState:     string(tfState),
		DestroyTarget:      "null_resource.res1",
	})
	if utilErr != nil {
		s.NoError(utilErr, "Expected no error while destroying resource")
		return
	}
	tfStateOutput := output.TerraformState
	tfStateJson, err = json.Parser().Unmarshal([]byte(tfStateOutput))
	s.Require().NoError(err, "Expected to unmarshal the Terraform state JSON")
	s.NotNil(tfStateJson["resources"], "Expected resources in the state file")
	s.Len(tfStateJson["resources"].([]any), 1, "Expected one resources in the state file")
	s.Equal("res2", tfStateJson["resources"].([]interface{})[0].(map[string]interface{})["name"], "Expected first resource name to be res2")
}
