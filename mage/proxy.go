package mage

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"
)

// Starts a TLS SNI reverse proxy server that exposes all Orchestrator services using the given address. This proxy
// assumes DNS is configured for the cluster domain so that all Orchestrator DNS records resolve to the given address.
// It is only useful to expose the Orchestrator services externally of the host this is executed on (e.g., onboarding an
// Edge Node). If listening on port 443, the process must be run with elevated privileges, either as root or with the
// capability to bind to privileged ports. This can be done by setting the `cap_net_bind_service` capability on the
// binary.
//
// Usage:
//
// 1. Compile the mage file: mage -compile ./static-magefile
// 2. Add the capability to bind to port 443: sudo setcap 'cap_net_bind_service=+ep' ./static-magefile
// 3. Run the proxy: ./static-magefile deploy:proxy <network-interface-IP>:443
//
// TODO: tinkerbell-nginx may not work due to it using TLS cipher suites that are not compatible with the default Go TLS
// configuration. Possibly using a pure TCP proxy instead of a TLS proxy just for tinkerbell-nginx would work.
func (Deploy) Proxy(ctx context.Context, addr string) error {
	decodedCert, decodedKey, err := OrchTLSCertAndKey(ctx)
	if err != nil {
		return fmt.Errorf("get certificate and key: %w", err)
	}

	certFile, err := os.CreateTemp("", "proxy-cert-")
	if err != nil {
		return fmt.Errorf("create certificate file: %w", err)
	}
	defer os.Remove(certFile.Name())

	if _, err := certFile.Write(decodedCert); err != nil {
		return fmt.Errorf("write certificate file: %w", err)
	}

	keyFile, err := os.CreateTemp("", "proxy-key-")
	if err != nil {
		return fmt.Errorf("create key file: %w", err)
	}
	defer os.Remove(keyFile.Name())

	if _, err := keyFile.Write(decodedKey); err != nil {
		return fmt.Errorf("write key file: %w", err)
	}

	handler, err := NewReverseProxyHandler()
	if err != nil {
		return fmt.Errorf("create handler: %w", err)
	}

	s := &http.Server{
		Addr:           addr,
		Handler:        handler,
		ReadTimeout:    time.Minute,
		WriteTimeout:   time.Minute,
		MaxHeaderBytes: http.DefaultMaxHeaderBytes,
	}

	fmt.Printf(
		"Started TLS reverse proxy on https://%s using certificate %s and key %s\n",
		addr,
		certFile.Name(),
		keyFile.Name(),
	)

	return s.ListenAndServeTLS(certFile.Name(), keyFile.Name())
}

// ReverseProxyHandler is an instance of httputil.ReverseProxy configured to forward HTTP requests to an upstream
// service. It uses the TLS SNI (Server Name Indication) to determine the target service.
type ReverseProxyHandler struct {
	Proxy *httputil.ReverseProxy
}

func NewReverseProxyHandler() (*ReverseProxyHandler, error) {
	return &ReverseProxyHandler{
		Proxy: &httputil.ReverseProxy{
			Rewrite: func(r *httputil.ProxyRequest) {
				// Set X-Forwarded-* headers to indicate that the request is being proxied.
				r.SetXForwarded()

				// Pass the request to the upstream service using the TLS servername since Orchestrator's
				// ingress will need this to route to the correct service. It is assumed that the upstream service is
				// listening on port 443 and is on the same host as the reverse proxy.
				r.SetURL(
					&url.URL{
						Scheme: "https",
						Host:   r.In.TLS.ServerName,
					},
				)
			},
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true, // TODO: Use a CA pool instead of skipping verification
				},
			},
			ErrorLog: log.New(os.Stderr, "Proxy: ", log.Lshortfile),
		},
	}, nil
}

// ServeHTTP is the HTTP handler method for the ReverseProxyHandler struct. It routes incoming HTTP requests to the
// single reverse proxy instance.
func (p *ReverseProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("Received request: Host=%s, ServerName=%s, URL=%s\n", r.Host, r.TLS.ServerName, r.URL)
	p.Proxy.ServeHTTP(w, r)
}
