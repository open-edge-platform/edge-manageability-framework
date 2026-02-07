// SPDX-FileCopyrightText: 2026 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package mage

import (
	"testing"
)

func TestDetermineUpgradePath(t *testing.T) {
	tests := []struct {
		name           string
		currentVersion string
		targetVersion  string
		expectedPath   []string
	}{
		{
			name:           "patch upgrade within 1.34",
			currentVersion: "v1.34.1+rke2r1",
			targetVersion:  "v1.34.3+rke2r1",
			expectedPath:   []string{"v1.34.3+rke2r1"},
		},
		{
			name:           "already at target version",
			currentVersion: "v1.34.3+rke2r1",
			targetVersion:  "v1.34.3+rke2r1",
			expectedPath:   []string{},
		},
		{
			name:           "upgrade from 1.30 to 1.34",
			currentVersion: "v1.30.14+rke2r2",
			targetVersion:  "v1.34.3+rke2r1",
			expectedPath:   []string{"v1.31.13+rke2r1", "v1.32.9+rke2r1", "v1.33.5+rke2r1", "v1.34.1+rke2r1", "v1.34.3+rke2r1"},
		},
		{
			name:           "unknown current version within same minor",
			currentVersion: "v1.34.2+rke2r1",
			targetVersion:  "v1.34.3+rke2r1",
			expectedPath:   []string{"v1.34.3+rke2r1"},
		},
		{
			name:           "very old version requiring multiple intermediate upgrades",
			currentVersion: "v1.29.10+rke2r1",
			targetVersion:  "v1.34.3+rke2r1",
			expectedPath:   []string{"v1.30.14+rke2r2", "v1.31.13+rke2r1", "v1.32.9+rke2r1", "v1.33.5+rke2r1", "v1.34.1+rke2r1", "v1.34.3+rke2r1"},
		},
		{
			name:           "target version not in list but in same minor",
			currentVersion: "v1.34.0+rke2r1",
			targetVersion:  "v1.34.5+rke2r1",
			expectedPath:   []string{"v1.34.5+rke2r1"},
		},
		{
			name:           "unsupported target minor version",
			currentVersion: "v1.34.1+rke2r1",
			targetVersion:  "v1.35.0+rke2r1",
			expectedPath:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := determineUpgradePath(tt.currentVersion, tt.targetVersion)
			
			if len(got) != len(tt.expectedPath) {
				t.Errorf("determineUpgradePath() got length %d, want %d\nGot: %v\nWant: %v", 
					len(got), len(tt.expectedPath), got, tt.expectedPath)
				return
			}
			
			for i := range got {
				if got[i] != tt.expectedPath[i] {
					t.Errorf("determineUpgradePath()[%d] = %v, want %v", i, got[i], tt.expectedPath[i])
				}
			}
		})
	}
}
