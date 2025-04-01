// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package pki

import (
	"bytes"
	"context"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/crypto/ocsp"
)

// Client can interface with CRLs and OCSP services hosted over HTTP.
type Client struct {
	httpCli *http.Client
}

// New returns a PKI client.
func New(httpCli *http.Client) (*Client, error) {
	if httpCli == nil {
		httpCli = &http.Client{}
	}

	return &Client{
		httpCli: httpCli,
	}, nil
}

// RevocationList returns the CRL from the given address.
func (c *Client) RevocationList(ctx context.Context, crlAddr string) (*x509.RevocationList, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, crlAddr, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := c.httpCli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	crl, err := x509.ParseRevocationList(bytes)
	if err != nil {
		return nil, fmt.Errorf("parse CRL: %w", err)
	}

	return crl, nil
}

// CertificateOCSPStatus returns the OCSP status for a certificate.
func (c *Client) CertificateOCSPStatus(
	ctx context.Context,
	ocspURL string,
	issuer, cert *x509.Certificate,
) (*ocsp.Response, error) {
	ocspReq, err := ocsp.CreateRequest(cert, issuer, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ocspURL, bytes.NewReader(ocspReq))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	req.Header.Add("Content-Type", "application/ocsp-request")
	req.Header.Add("Accept", "application/ocsp-response")

	resp, err := c.httpCli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	ocspResp, err := ocsp.ParseResponse(body, issuer)
	if err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return ocspResp, nil
}

// ContainsCert returns true if a certificate is present in the CRL.
func ContainsCert(crl *x509.RevocationList, cert *x509.Certificate) bool {
	for _, revokedCert := range crl.RevokedCertificates {
		if revokedCert.SerialNumber.Cmp(cert.SerialNumber) == 0 {
			return true
		}
	}
	return false
}
