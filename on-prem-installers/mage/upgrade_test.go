// SPDX-FileCopyrightText: 2026 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package mage

import (
"fmt"
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

func TestDetermineUpgradePathDebug(t *testing.T) {
currentVersion := "v1.34.1+rke2r1"
targetVersion := "v1.34.3+rke2r1"

fmt.Printf("Testing upgrade from %s to %s\n", currentVersion, targetVersion)
path := determineUpgradePath(currentVersion, targetVersion)
fmt.Printf("Got path: %v (length: %d)\n", path, len(path))

if len(path) == 0 {
t.Errorf("Expected non-empty upgrade path from %s to %s", currentVersion, targetVersion)
}
}

func TestDetermineUpgradePathUnknownVersion(t *testing.T) {
// Test with a version not in the hardcoded list
currentVersion := "v1.34.2+rke2r1"
targetVersion := "v1.34.3+rke2r1"

fmt.Printf("Testing upgrade from %s to %s\n", currentVersion, targetVersion)
path := determineUpgradePath(currentVersion, targetVersion)
fmt.Printf("Got path: %v (length: %d)\n", path, len(path))

// This should still work because both are in 1.34 minor
if len(path) == 0 {
t.Errorf("Expected non-empty upgrade path from %s to %s", currentVersion, targetVersion)
}
}

func TestDetermineUpgradePathVeryOldVersion(t *testing.T) {
// Test with a version older than the hardcoded list
currentVersion := "v1.29.10+rke2r1"
targetVersion := "v1.34.3+rke2r1"

fmt.Printf("Testing upgrade from %s to %s\n", currentVersion, targetVersion)
path := determineUpgradePath(currentVersion, targetVersion)
fmt.Printf("Got path: %v (length: %d)\n", path, len(path))

// This might return empty because 1.29 is not in the list
}

func TestDetermineUpgradePathSameMinorNotInList(t *testing.T) {
// What if both versions are in the same minor but neither is in the list?
currentVersion := "v1.34.0+rke2r1"
targetVersion := "v1.34.5+rke2r1"

fmt.Printf("Testing upgrade from %s to %s\n", currentVersion, targetVersion)
path := determineUpgradePath(currentVersion, targetVersion)
fmt.Printf("Got path: %v (length: %d)\n", path, len(path))

// Since 1.34.5 is not in the list, this should return the versions in 1.34 range
}

func TestDetermineUpgradePathUnsupportedMinor(t *testing.T) {
// Test upgrading to a minor version not in the list
currentVersion := "v1.34.1+rke2r1"
targetVersion := "v1.35.0+rke2r1"

fmt.Printf("Testing upgrade from %s to %s\n", currentVersion, targetVersion)
path := determineUpgradePath(currentVersion, targetVersion)
fmt.Printf("Got path: %v (length: %d)\n", path, len(path))

// Since 1.35 is not in the hardcoded list, this should return empty
if len(path) != 0 {
t.Errorf("Expected empty upgrade path for unsupported target version, got: %v", path)
}
}

func TestDetermineUpgradePathTargetInList(t *testing.T) {
// Test that we can upgrade to any version in the list from a patch version
currentVersion := "v1.34.2+rke2r1" // Not in list
targetVersion := "v1.34.3+rke2r1"   // In list

path := determineUpgradePath(currentVersion, targetVersion)

expected := []string{"v1.34.3+rke2r1"}
if len(path) != len(expected) {
t.Errorf("Expected path %v, got %v", expected, path)
}
}
