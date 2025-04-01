// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package orchestrator_test

/* Functions and constants that are common to catalog and network tests */

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	. "github.com/onsi/gomega"
)

const (
	testUsername       = "e2etestuser-edge-mgr"
	testOrg            = "e2eTestOrg"  // name of pre-created test org
	testProject        = "e2eTestProj" // name of pre-created test project
	catalogApiVersion  = "v3"
	ADMApiVersion      = "v1"
	ignoreRESTResponse = true
	checkRESTResponse  = false
)

var apiBaseURL = "https://api." + serviceDomainWithPort

func getAccessToken(c *http.Client, username string, password string) string {
	data := url.Values{}
	data.Set("client_id", "system-client")
	data.Set("username", username)
	data.Set("password", password)
	data.Set("grant_type", "password")
	req, err := http.NewRequestWithContext(
		context.TODO(),
		http.MethodPost,
		"https://keycloak."+serviceDomainWithPort+"/realms/master/protocol/openid-connect/token",
		strings.NewReader(data.Encode()),
	)
	Expect(err).ToNot(HaveOccurred())
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.Do(req) //nolint: bodyclose
	Expect(err).ToNot(HaveOccurred())
	defer resp.Body.Close()
	Expect(resp.StatusCode).To(Equal(http.StatusOK), func() string {
		b, err := io.ReadAll(resp.Body)
		Expect(err).ToNot(HaveOccurred())
		return fmt.Sprintf("error accessing https://keycloak.%s/realms/master/protocol/openid-connect/token %s",
			serviceDomainWithPort, string(b))
	})
	rawTokenData, err := io.ReadAll(resp.Body)
	Expect(err).ToNot(HaveOccurred())
	tokenData := map[string]interface{}{}
	err = json.Unmarshal(rawTokenData, &tokenData)
	Expect(err).ToNot(HaveOccurred())

	accessToken := tokenData["access_token"].(string)
	Expect(accessToken).To(Not(ContainSubstring(`named cookie not present`)))
	return accessToken
}

// doREST uses the REST API to perform a REST operation.
func doREST(
	ctx context.Context,
	c *http.Client,
	method string,
	url string,
	accessToken string,
	body io.Reader,
	expectedStatus int,
	ignoreResponse bool,
) *http.Response {
	req, err := http.NewRequestWithContext(ctx, method,
		url,
		body)
	Expect(err).ToNot(HaveOccurred())
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Set("User-Agent", "orchestrator-cli")
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if !ignoreResponse {
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(expectedStatus), func() string {
			b, err := io.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			return fmt.Sprintf("error on %s %s %s",
				method, url, string(b))
		})
	}
	return resp
}
