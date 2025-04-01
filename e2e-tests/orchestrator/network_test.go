// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package orchestrator_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	util "github.com/open-edge-platform/edge-manageability-framework/mage"
)

const (
	netApiVersion = "v1"
)

type NetworkSpec struct {
	Description string `json:"description,omitempty"`
	Type        string `json:"type,omitempty"`
}

// Network is the JSON representation of network.
type Network struct {
	Name string      `json:"name,omitempty"`
	Spec NetworkSpec `json:"spec,omitempty"`
}

// networks is the JSON representation of a list of networks.
type networks []Network

// listNetworks uses the REST API to list the networks.
func listNetworks(ctx context.Context, c *http.Client, accessToken string,
	project string, expectedStatus int,
) networks {
	url := fmt.Sprintf("%s/%s/projects/%s/networks", apiBaseURL, netApiVersion, project)
	resp := doREST(ctx, c, http.MethodGet, url, accessToken, //nolint: bodyclose
		nil, expectedStatus, checkRESTResponse)
	defer resp.Body.Close()
	Expect(resp.StatusCode).To(Equal(expectedStatus), func() string {
		b, err := io.ReadAll(resp.Body)
		Expect(err).ToNot(HaveOccurred())
		return fmt.Sprintf("error on GET .../networks %s",
			string(b))
	})
	var networksResp networks
	err := json.NewDecoder(resp.Body).Decode(&networksResp)
	Expect(err).ToNot(HaveOccurred())
	return networksResp
}

// listAndFindNetwork uses the REST API to list the networks and find a specific one.
func listAndFindNetwork(ctx context.Context, c *http.Client, accessToken string,
	project string, regName string,
) *Network {
	// TODO: replace list-and-search with a REST API call that filters by name.
	networks := listNetworks(ctx, c, accessToken, project, http.StatusOK)
	for i := 0; i < len(networks); i++ {
		if networks[i].Name == regName {
			return &networks[i]
		}
	}
	return nil
}

// createNetwork uses the REST API to create a Network.
func createNetwork(ctx context.Context, c *http.Client, projectName, accessToken string, name string, network NetworkSpec) {
	networkBody, err := json.Marshal(network)
	Expect(err).ToNot(HaveOccurred())
	url := fmt.Sprintf("%s/%s/projects/%s/networks/%s", apiBaseURL, netApiVersion, projectName, name)
	fmt.Printf("%s\n", url)
	resp := doREST(ctx, c, http.MethodPut, url,
		accessToken, bytes.NewReader(networkBody), http.StatusOK, checkRESTResponse)
	defer resp.Body.Close()
}

// deleteNetwork uses the REST API to delete a Network.
func deleteNetwork(ctx context.Context, c *http.Client, projectName, accessToken string, networkName string, ignoreResponse bool) {
	url := fmt.Sprintf("%s/%s/projects/%s/networks/%s", apiBaseURL, netApiVersion, projectName, networkName)
	resp := doREST(ctx, c, http.MethodDelete, url,
		accessToken, nil, http.StatusOK, ignoreResponse)
	defer resp.Body.Close()
}

var _ = Describe("Network API Tests", Label("orchestrator-integration"), func() {
	var c *http.Client
	var cancel context.CancelFunc
	var ctx context.Context

	testPassword := func() string {
		pass, err := util.GetDefaultOrchPassword()
		if err != nil {
			log.Fatal(err)
		}
		return pass
	}()

	BeforeEach(func() {
		c = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
		}
		ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	})

	AfterEach(func() {
		cancel()
	})

	Describe("Network REST API CRUD Test", Ordered, func() {
		When("Using the network REST API", Ordered, func() {
			const netName = "testnet1"
			It("should Get a token from KeyCloak", func() {
				accessToken = getAccessToken(c, testUsername, testPassword)
				Expect(accessToken).To(Not(ContainSubstring(`named cookie not present`)))
			})

			It("should ensure no network is leftover from a previous run", func() {
				// Delete network
				deleteNetwork(ctx, c, testProject, accessToken, netName, ignoreRESTResponse)
				Expect(listAndFindNetwork(ctx, c, accessToken, testProject, netName)).To(BeNil())
			})

			It("should create a network", func() {
				createNetwork(ctx, c, testProject, accessToken, netName, NetworkSpec{
					Description: "test network",
					Type:        "application-mesh",
				})
			})

			It("should determine that the new network was created", func() {
				networks := listNetworks(ctx, c, accessToken, testProject, http.StatusOK)
				found := false

				for _, net := range networks {
					if net.Name == netName {
						found = true
						Expect(net.Spec.Description).To(Equal("test network"))
						Expect(net.Spec.Type).To(Equal("application-mesh"))
					}
				}
				Expect(found).To(BeTrue())
			})

			It("should delete the network and ensure it is gone", func() {
				// Delete network
				deleteNetwork(ctx, c, testProject, accessToken, netName, checkRESTResponse)
				Expect(listAndFindNetwork(ctx, c, accessToken, testProject, netName)).To(BeNil())
			})
		})
	})
})
