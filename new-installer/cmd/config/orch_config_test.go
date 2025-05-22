package main

import (
	"testing"
)

// TODO: Add unit tests for all validation functions

func TestValidateAwsRegion(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expectErr bool
	}{
		{
			name:      "Valid AWS region",
			input:     "us-west-2",
			expectErr: false,
		},
		{
			name:      "Invalid AWS region - missing number",
			input:     "us-west",
			expectErr: true,
		},
		{
			name:      "Invalid AWS region - missing hyphen",
			input:     "uswest2",
			expectErr: true,
		},
		{
			name:      "Invalid AWS region - extra characters",
			input:     "us-west-2a",
			expectErr: true,
		},
		{
			name:      "Invalid AWS region - empty string",
			input:     "",
			expectErr: true,
		},
		{
			name:      "Invalid AWS region - special characters",
			input:     "us-west-@",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAwsRegion(tt.input)
			if (err != nil) != tt.expectErr {
				t.Errorf("validateAwsRegion(%q) error = %v, expectErr = %v", tt.input, err, tt.expectErr)
			}
		})
	}
}
func TestValidateSimpleMode(t *testing.T) {
	tests := []struct {
		name      string
		input     []string
		expectErr bool
	}{
		{
			name:      "Valid - Only FPS enabled",
			input:     []string{"fps"},
			expectErr: false,
		},
		{
			name:      "Valid - FPS and UI with EIM enabled",
			input:     []string{"fps", "ui", "eim"},
			expectErr: false,
		},
		{
			name:      "Valid - FPS and UI with CO enabled",
			input:     []string{"fps", "ui", "co"},
			expectErr: false,
		},
		{
			name:      "Valid - FPS and UI with AO enabled",
			input:     []string{"fps", "ui", "ao"},
			expectErr: false,
		},
		{
			name:      "Invalid - FPS not enabled",
			input:     []string{"ui", "eim"},
			expectErr: true,
		},
		{
			name:      "Invalid - UI enabled without EIM, CO, or AO",
			input:     []string{"fps", "ui"},
			expectErr: true,
		},
		{
			name:      "Valid - FPS and multiple packages enabled",
			input:     []string{"fps", "ui", "eim", "co", "ao"},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSimpleMode(tt.input)
			if (err != nil) != tt.expectErr {
				t.Errorf("validateSimpleMode(%v) error = %v, expectErr = %v", tt.input, err, tt.expectErr)
			}
		})
	}
}

func TestValidateAdvancedMode(t *testing.T) {
	// TODO: placeholder for advanced mode validation
	return
}
