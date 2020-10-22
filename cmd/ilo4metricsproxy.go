package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"golang.org/x/net/publicsuffix"
	"ilo4-metrics-proxy/pkg/ilo4"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
)

func main() {
	// Logger
	zapLog, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	log := zapr.NewLogger(zapLog)

	// HTTP
	httpClient, err := newHttpClient([]byte("-----BEGIN CERTIFICATE-----\nMIICujCCAiOgAwIBAgIISbJSyZ/4+SowDQYJKoZIhvcNAQELBQAwgYoxJjAkBgNV\nBAMMHURlZmF1bHQgSXNzdWVyIChEbyBub3QgdHJ1c3QpMSMwIQYDVQQKDBpIZXds\nZXR0IFBhY2thcmQgRW50ZXJwcmlzZTEMMAoGA1UECwwDSVNTMRAwDgYDVQQHDAdI\nb3VzdG9uMQ4wDAYDVQQIDAVUZXhhczELMAkGA1UEBhMCVVMwHhcNMTgwODAzMTkx\nOTQ4WhcNMzMwODAyMTkxOTQ4WjB8MRgwFgYDVQQDDA9pbG8ubWR2b3Jhay5vcmcx\nIzAhBgNVBAoMGkhld2xldHQgUGFja2FyZCBFbnRlcnByaXNlMQwwCgYDVQQLDANJ\nU1MxEDAOBgNVBAcMB0hvdXN0b24xDjAMBgNVBAgMBVRleGFzMQswCQYDVQQGEwJV\nUzCBnzANBgkqhkiG9w0BAQEFAAOBjQAwgYkCgYEAtyrgsEQ1s+ktshr9GurP/nSG\nU+flShBZ5my2nSyJxsoFS03XfIRBnH5O9IFs3oyFYLE+GMYoVLjIcwuEz8NLFfsM\n1TSviSmv8Ox9+obHw2I1oT8Yd5/KpdazZkFIHTuCSfFh3zU2CZEMk4ht40PTPjJo\nltVsaAsFBTvfeDxxSKMCAwEAAaM2MDQwCwYDVR0PBAQDAgWgMAkGA1UdIwQCMAAw\nGgYDVR0RBBMwEYIPaWxvLm1kdm9yYWsub3JnMA0GCSqGSIb3DQEBCwUAA4GBACLv\nfogJ7GX+ZjV3A2t4sOTzTm0gujFLqUfFkbMXGJOoumxM2NcHKz2gFdGQi7MKe4wC\noJ8PXGcY94AyadPo6wgwQtjtqvqTxEKR/1ND13XZYTJMc8cW3A/0u818lmaEecIn\nNz14WiJI7sxOdeduy18k2E63tsTi9HqF4KnZbcCz\n-----END CERTIFICATE-----\n"))

	// Client
	iloClient := &ilo4.Ilo4Client{
		Log:    log,
		Client: httpClient,
		Url:    "https://ilo.mdvorak.org",
		CredentialsProvider: func() (io.Reader, error) {
			return strings.NewReader(`{"method":"login","user_login":"metrics","password":""}`), nil
		},
	}

	// Test
	err = iloClient.DoGetTempratures(context.TODO(), true)
	if err != nil {
		panic(err)
	}
}

func newHttpClient(serverCert []byte) (*http.Client, error) {
	// Certificates
	certs := x509.NewCertPool()
	certs.AppendCertsFromPEM(serverCert)

	// TLS
	tlsConfig := &tls.Config{
		RootCAs: certs,
	}

	// Cookies are needed
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
