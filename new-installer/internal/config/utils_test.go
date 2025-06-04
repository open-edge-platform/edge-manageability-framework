// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
package config_test

import (
	"testing"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"github.com/stretchr/testify/suite"
)

type UtilsTestSuite struct {
	suite.Suite
}

func TestUtils(t *testing.T) {
	suite.Run(t, new(UtilsTestSuite))
}
func (s *UtilsTestSuite) TestCommaSeparatedToSlice() {
	tests := []struct {
		input    string
		expected []string
	}{
		{"", []string{}},
		{"one", []string{"one"}},
		{"one,two", []string{"one", "two"}},
		{"one, two", []string{"one", "two"}},
		{" one , two , three ", []string{"one", "two", "three"}},
		{",one,,two,", []string{"", "one", "", "two", ""}},
		{"  ,  ,  ", []string{"", "", ""}},
	}

	for _, tt := range tests {
		result := config.CommaSeparatedToSlice(tt.input)
		s.Equal(tt.expected, result, "input: %q", tt.input)
	}
}

func (s *UtilsTestSuite) TestSliceToCommaSeparated() {
	tests := []struct {
		input    []string
		expected string
	}{
		{nil, ""},
		{[]string{}, ""},
		{[]string{"one"}, "one"},
		{[]string{"one", "two"}, "one, two"},
		{[]string{"one", "two", "three"}, "one, two, three"},
		{[]string{"", "one", "", "two", ""}, ", one, , two, "},
		{[]string{"  ", " "}, "  ,  "},
	}

	for _, tt := range tests {
		result := config.SliceToCommaSeparated(tt.input)
		s.Equal(tt.expected, result, "input: %#v", tt.input)
	}
}

func (s *UtilsTestSuite) TestSerializeAndDeserialize() {
	data := []byte(`---
version: 1
provider: "aws"
global:
  orchName: "test"
`)

	obj := config.OrchInstallerConfig{}
	err := config.DeserializeFromYAML(&obj, data)
	if !s.NoError(err, "failed to unmarshal yaml") {
		return
	}
	s.Equal(1, obj.Version)
	s.Equal("aws", obj.Provider)
	s.Equal("test", obj.Global.OrchName)
	data2, err := config.SerializeToYAML(obj)
	s.NoError(err, "failed to marshal yaml")

	obj2 := config.OrchInstallerConfig{}
	err = config.DeserializeFromYAML(&obj2, data2)
	s.NoError(err, "failed to unmarshal yaml")
	s.Equal(obj.Version, obj2.Version)
	s.Equal(obj.Provider, obj2.Provider)
	s.Equal(obj.Global.OrchName, obj2.Global.OrchName)
}
