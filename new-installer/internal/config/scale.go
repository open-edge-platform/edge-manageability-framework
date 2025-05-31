// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"slices"
)

type Scale int

const (
	Scale10    Scale = 10
	Scale100   Scale = 100
	Scale500   Scale = 500
	Scale1000  Scale = 1000
	Scale10000 Scale = 10000
)

// IsValid checks if a Scale value is one of the defined constants
func (s Scale) IsValid() bool {
	return slices.Contains(ValidScales(), s)
}

func ValidScales() []Scale {
	return []Scale{Scale10, Scale100, Scale500, Scale1000, Scale10000}
}
