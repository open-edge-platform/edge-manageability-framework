// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"testing"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestValidScales(t *testing.T) {
	// Test that ValidScales returns all expected scale values
	expected := []config.Scale{
		config.Scale50,
		config.Scale100,
		config.Scale500,
		config.Scale1000,
	}

	result := config.ValidScales()

	// Check that the result contains the correct number of elements
	assert.Equal(t, len(expected), len(result), "ValidScales should return the correct number of scale values")

	// Check that the result contains all expected scale values
	for _, scale := range expected {
		assert.Contains(t, result, scale, "ValidScales should contain scale %d", scale)
	}

	// Ensure the values are in the expected order
	for i, scale := range expected {
		assert.Equal(t, scale, result[i], "ValidScales should return scales in the correct order")
	}
}

func TestScaleIsValid(t *testing.T) {
	testCases := []struct {
		name     string
		scale    config.Scale
		expected bool
	}{
		{"Scale50", config.Scale50, true},
		{"Scale100", config.Scale100, true},
		{"Scale500", config.Scale500, true},
		{"Scale1000", config.Scale1000, true},
		{"Invalid scale - 0", config.Scale(0), false},
		{"Invalid scale - 2000", config.Scale(2000), false},
		{"Invalid scale - -10", config.Scale(-10), false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.scale.IsValid()
			assert.Equal(t, tc.expected, result)
		})
	}
}
