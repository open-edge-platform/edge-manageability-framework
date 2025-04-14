package mage

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// Starts a TLS reverse proxy server that exposes all Orchestrator services over a single port on the given address.
func (Deploy) Proxy(ctx context.Context, addr string) error {
	decodedCert, decodedKey, err := OrchTLSCertAndKey(ctx)
	if err != nil {
		return fmt.Errorf("get certificate and key: %w", err)
	}

	certFile, err := os.CreateTemp("", "tls-proxy-cert-")
	if err != nil {
		return fmt.Errorf("create certificate file: %w", err)
	}
	defer os.Remove(certFile.Name())

	if _, err := certFile.Write(decodedCert); err != nil {
		return fmt.Errorf("write certificate file: %w", err)
	}

	keyFile, err := os.CreateTemp("", "tls-proxy-key-")
	if err != nil {
		return fmt.Errorf("create key file: %w", err)
	}
	defer os.Remove(keyFile.Name())

	if _, err := keyFile.Write(decodedKey); err != nil {
		return fmt.Errorf("write key file: %w", err)
	}

	handler, err := NewReverseProxyHandler(
		// TODO: Make configurable
		map[string]string{
			"argocd.":           "192.168.99.20",
			"tinkerbell-nginx.": "192.168.99.40",
			"traefik.":          "192.168.99.30",
		},
	)
	if err != nil {
		return fmt.Errorf("create handler: %w", err)
	}

	s := &http.Server{
		Addr:           addr,
		Handler:        handler,
		ReadTimeout:    15 * time.Second,
		WriteTimeout:   15 * time.Second,
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

// OrchTLSCertAndKey returns the Orchestrator's TLS certificate and key.
func OrchTLSCertAndKey(ctx context.Context) ([]byte, []byte, error) {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		return nil, nil, fmt.Errorf("KUBECONFIG environment variable is not set")
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, nil, fmt.Errorf("load Kubernetes config from KUBECONFIG: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, fmt.Errorf("create Kubernetes client: %w", err)
	}

	secret, err := clientset.CoreV1().Secrets("orch-gateway").Get(ctx, "tls-orch", metav1.GetOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get secret 'tls-orch': %w", err)
	}

	cert, certExists := secret.Data["tls.crt"]
	key, keyExists := secret.Data["tls.key"]

	if !certExists || !keyExists {
		return nil, nil, fmt.Errorf("certificate or key not found in secret 'tls-orch'")
	}

	return cert, key, nil
}

type ReverseProxyHandler struct {
	Routes map[string]*httputil.ReverseProxy
}

func NewReverseProxyHandler(routeIPs map[string]string) (*ReverseProxyHandler, error) {
	routes := make(map[string]*httputil.ReverseProxy, 3)

	for name, ip := range routeIPs {
		proxy := httputil.NewSingleHostReverseProxy(
			&url.URL{
				Scheme: "https",
				Host:   ip + ":443",
			},
		)

		proxy.Transport = &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}

		routes[name] = proxy
	}

	return &ReverseProxyHandler{
		Routes: routes,
	}, nil
}

// ServeHTTP is the HTTP handler method for the ReverseProxyHandler struct.
// It routes incoming HTTP requests to the appropriate backend service based on the
// ServerName field in the TLS configuration of the request.
func (p *ReverseProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("Received request: Host=%s, ServerName=%s, URL=%s\n", r.Host, r.TLS.ServerName, r.URL)

	switch {
	case strings.HasPrefix(r.TLS.ServerName, "argocd."):
		fmt.Printf("Routing to argocd backend\n")
		p.Routes["argocd."].ServeHTTP(w, r)

	case strings.HasPrefix(r.TLS.ServerName, "tinkerbell-nginx."):
		fmt.Printf("Routing to tinkerbell-nginx backend\n")
		p.Routes["tinkerbell-nginx."].ServeHTTP(w, r)

	default:
		fmt.Printf("Routing to traefik backend\n")
		p.Routes["traefik."].ServeHTTP(w, r)
	}
}
