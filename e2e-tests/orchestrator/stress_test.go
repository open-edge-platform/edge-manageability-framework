// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package orchestrator_test

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bitfield/script"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	numRequests = 300
)

var _ = Describe("Orchestrator stress test", Label("stress-test"), func() {
	var cli *http.Client

	BeforeEach(func() {
		cli = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
		}

		fmt.Printf("serviceDomain: %v\n", serviceDomain)
	})

	Describe("Release Service Token endpoint", Label("stress-test"), func() {
		releaseTokenURL := "https://release." + serviceDomainWithPort + "/token"
		It("should send many requests to the server endpoint", func() {
			log.Println("################# stress test started...")
			statusMap := make(map[int]int)
			token := getKeycloakJWT(cli, "all-groups-example-user")
			for i := 0; i < numRequests; i++ {
				statusCode, err := stressFn(releaseTokenURL, token)
				Expect(err).ToNot(HaveOccurred())
				statusMap[statusCode] += 1
			}
			log.Printf("================= test results: %+v", statusMap)
		})
	})

	Describe("Curl stress test", Label("stress-test"), func() {
		It("Should Limit requests using cipher TLS_DHE_RSA_WITH_AES_256_GCM_SHA384", func() {
			cdnURL := "https://tinkerbell-nginx." + serviceDomain + "/tink-stack/boot.ipxe"
			cafilepath := "/tmp/cluster_ca.crt"
			cacert := "curl https://" + serviceDomain + "/boots/ca.crt -o " + cafilepath
			// get the ca cert so testing can be done
			_, err := script.NewPipe().Exec(cacert).String()
			messages := make(chan string)
			Expect(err).ToNot(HaveOccurred())

			/* test sends 1300 requests at endpoint in parallel
			start time and end time are taken and compared to 200 ok responses
			test assumes nginx rate limit is 1 request/s
			curl is needed for cipher DHE-RSA-AES256-GCM-SHA384 */
			start := time.Now()

			cmd := "curl --cacert " + cafilepath + " -s --noproxy \"*\" " +
				" --tlsv1.2 --tls-max 1.2 --ciphers DHE-RSA-AES256-GCM-SHA384 " +
				"-o /dev/null -w \"%{http_code}\" " + cdnURL + ""

			var wg sync.WaitGroup
			for i := 1; i <= 1300; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					req, _ := script.NewPipe().Exec(cmd).String()
					messages <- req + ","
				}(i)
			}
			go func() {
				wg.Wait()
				close(messages)
			}()
			curl_output := ""
			for msg := range messages {
				curl_output += msg
			}

			res_collection := strings.Split(curl_output, ",")
			res200 := 0
			res429 := 0
			for i := 0; i < len(res_collection); i++ {
				// count number of 200 and 503 response codes
				if res_collection[i] == "200" {
					res200 += 1
				} else if res_collection[i] == "429" {
					res429 += 1
				}
			}
			time_difference := time.Since(start).Seconds()
			log.Printf("TIME in seconds from start of test %.f", time_difference)
			log.Printf("\n 429 responses: %d VS 200 responses: %d \n", res429, res200)
			percentage_200_429 := float32(res200) / (float32(res429) + float32(res200))
			log.Printf("Percentage of 200's/429's %f", percentage_200_429)
			Expect(err).ToNot(HaveOccurred())
			/* buffer of 10 responses needed as nginx is slow at applying limiting */
			Expect(time_difference+10 > float64(res200)).To(BeTrue())
			Expect(res429).To(BeNumerically(">", 1000))
		})
	})
})

func stressFn(releaseTokenURL, token string) (int, error) {
	req, err := http.NewRequest("GET", releaseTokenURL, nil)
	if err != nil {
		return 0, err
	}
	cli := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}
	req.Header.Add("Authorization", "Bearer "+token)
	resp, err := cli.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}
