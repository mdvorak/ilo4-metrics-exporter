package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/go-logr/zapr"
	"github.com/namsral/flag"
	"go.uber.org/zap"
	"golang.org/x/net/publicsuffix"
	"ilo4-metrics-proxy/pkg/ilo4"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"os"
)

func main() {
	// Logger
	zapLog, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	log := zapr.NewLogger(zapLog)

	// Get configuration
	var url string
	flag.StringVar(&url, "ilo-url", "", "iLO server base URL, e.g. https://ilo.example.com")

	var certificatePath string
	flag.StringVar(&certificatePath, "ilo-certificate-path", "", "path to a iLO server certificate, in PEM format")

	var credentialsPath string
	flag.StringVar(&credentialsPath, "ilo-credentials-path", "", "path to a valid JSON with server credentials")

	var labels string
	flag.StringVar(&labels, "prometheus-labels", "", "comma-separated list of labels, in key:value format, added to all prometheus metrics")

	flag.Parse()

	// Validate flags
	if url == "" {
		panic(fmt.Errorf("ilo-url not set"))
	}
	if credentialsPath == "" {
		panic(fmt.Errorf("ilo-credentials-path not set"))
	}

	// Read certificate
	log.Info("reading certificate", "path", certificatePath)
	serverCert, err := ioutil.ReadFile(certificatePath)
	if err != nil {
		panic(err)
	}

	// HTTP
	httpClient, err := newHttpClient(serverCert)
	if err != nil {
		panic(err)
	}

	// Client
	log.Info("initializing iLO4 client", "url", url)
	iloClient := &ilo4.Client{
		Log:    log.WithName("ilo4-client"),
		Client: httpClient,
		Url:    url,
		CredentialsProvider: func() (io.Reader, error) {
			log.Info("reading credentials", "path", credentialsPath)
			return os.Open(credentialsPath)
		},
	}

	// Start
	// TODO http server
	log.Info("started")

	// Test
	temps, err := iloClient.GetTemperatures(context.TODO())
	if err != nil {
		panic(err)
	}

	fmt.Println(temps)
}

func newHttpClient(serverCert []byte) (*http.Client, error) {
	// TLS
	tlsConfig := &tls.Config{}

	if serverCert != nil {
		certs := x509.NewCertPool()
		certs.AppendCertsFromPEM(serverCert)

		tlsConfig.RootCAs = certs
	}

	// Cookies are needed for session
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return nil, err
	}

	// Create client
	client := &http.Client{
		Transport: &http.Transport{TLSClientConfig: tlsConfig},
		Jar:       jar,
	}

	// Success
	return client, nil
}
