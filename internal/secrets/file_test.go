// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package secrets_test

import (
	"io"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/open-edge-platform/edge-manageability-framework/internal/secrets"
)

type mockReaderWriter struct {
	data []byte
}

func (m *mockReaderWriter) Write(p []byte) (n int, err error) {
	m.data = p
	return len(p), nil
}

func (m *mockReaderWriter) Read(p []byte) (n int, err error) {
	copy(p, m.data)
	return len(m.data), io.EOF
}

var _ = Describe("File Secrets Manager", func() {
	var (
		reader *mockReaderWriter
		writer *mockReaderWriter
	)

	Context("Secrets manager", func() {
		It("should return the secret", func() {
			reader = &mockReaderWriter{
				data: []byte("mockSecret"),
			}

			fs := &secrets.FileSaver{}
			result, err := fs.GetSecret(reader, "mockSecret")
			Expect(result).To(Equal("mockSecret"))
			Expect(err).ToNot(HaveOccurred())
		})

		It("should save the secret", func() {
			writer = &mockReaderWriter{
				data: []byte("mockSecret"),
			}

			fs := &secrets.FileSaver{}
			err := fs.SaveSecret(writer, "mockSecret")
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
