// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"io"
)

// wrapper Vtruct that will delegate "save file" task to our interface
type FileSaver struct{}

func NewFileSaver() *FileSaver {
	return &FileSaver{}
}

func (f *FileSaver) SaveSecret(r io.Writer, secret string) error {
	_, err := r.Write([]byte(secret))
	if err != nil {
		return err
	}
	return nil
}

func (f *FileSaver) GetSecret(r io.Reader, name string) (string, error) {
	secret, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(secret), nil
}
