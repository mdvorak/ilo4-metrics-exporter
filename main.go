package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/namsral/flag"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"golang.org/x/net/publicsuffix"
	"gopkg.in/fsnotify.v1"
	"ilo4-metrics-proxy/pkg/ilo4"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"os"
	"time"
)

func newZapLogger(production bool) (*zap.Logger, error) {
	if production {
		return zap.NewProduction()
	} else {
		return zap.NewDevelopment()
	}
}

func main() {
	var err error

	// Get configuration
	var production bool
	flag.BoolVar(&production, "production", false, "production logging format")

	var listen string
	flag.StringVar(&listen, "listen", ":2112", "address and port server should listen to")

	var url string
	flag.StringVar(&url, "ilo-url", "", "iLO server base URL, e.g. https://ilo.example.com")

	var certificatePath string
	flag.StringVar(&certificatePath, "ilo-certificate-path", "", "path to a iLO server certificate, in PEM format")

	var credentialsPath string
	flag.StringVar(&credentialsPath, "ilo-credentials-path", "", "path to a valid JSON with server credentials")

	var noLoginVerify bool
	flag.BoolVar(&noLoginVerify, "no-login-verify", false, "skip login credentials verification on start")

	flag.Parse()

	// Logger
	zapLog, err := newZapLogger(production)
	if err != nil {
		panic(err)
	}
	log := zapr.NewLogger(zapLog)

	// Validate flags
	if url == "" {
		panic(fmt.Errorf("ilo-url not set"))
	}
	if credentialsPath == "" {
		panic(fmt.Errorf("ilo-credentials-path not set"))
	}

	// HTTP
	httpClient, err := newHttpClient(log, certificatePath)
	if err != nil {
		panic(err)
	}

	// Client
	log.Info("initializing iLO4 client", "url", url)
	iloClient := ilo4.NewClient(log.WithName("ilo4-client"), httpClient, url, credentialsPath)

	// Try login (tests credentials file), so app does not start with invalid credentials at all
	if !noLoginVerify {
		err = iloClient.Login(context.Background())
		if err != nil {
			panic(fmt.Errorf("login failed, cannot start: %w", err))
		}
	}

	// Watch certificates
	err = watchCertificateChanges(log, certificatePath, iloClient)
	if err != nil {
		log.Error(err, "failed to setup filesystem watcher, certificate updates won't be available")
	} else {
		log.Info("watching certificate for changes", "path", certificatePath)
	}

	// Metrics
	prometheus.MustRegister(ilo4.NewTemperatureMetrics(iloClient))
	prometheus.MustRegister(iloClient.LoginCounts)

	// Start
	http.HandleFunc("/health", healthHandler)
	http.Handle("/metrics", promhttp.Handler())

	log.Info("listening on " + listen)
	if err := http.ListenAndServe(listen, nil); err != nil {
		panic(err)
	}
}

func healthHandler(writer http.ResponseWriter, _ *http.Request) {
	_, _ = writer.Write([]byte(time.Now().String()))
}

func newHttpClient(log logr.Logger, certificatePath string) (*http.Client, error) {
	// TLS
	tlsConfig := &tls.Config{}

	if certificatePath != "" {
		log.Info("reading certificate", "path", certificatePath)
		serverCert, err := ioutil.ReadFile(certificatePath)
		if err != nil {
			return nil, err
		}

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

func watchCertificateChanges(log logr.Logger, certificatePath string, iloClient *ilo4.Client) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	lastHash, err := fileHash(certificatePath)
	if err != nil {
		log.V(1).Error(err, "failed get certificate file hash", "path", certificatePath)
	}

	// Async certificate updates
	go func() {
		for {
			select {
			case _ = <-watcher.Events:
				// Get current checksum
				hash, err := fileHash(certificatePath)
				if err != nil {
					log.V(1).Error(err, "failed get certificate file hash", "path", certificatePath)
				}

				// Only if hash changed
				if !bytes.Equal(hash, lastHash) || hash == nil {
					log.Info("server certificate changed")
					if httpClient, err := newHttpClient(log, certificatePath); err != nil {
						log.Error(err, "failed to replace http client with new certificate")
					} else {
						// Replace client with new certificate
						iloClient.Client = httpClient
						lastHash = hash
					}
				}

			case err := <-watcher.Errors:
				log.Error(err, "filesystem watcher failed")
			}
		}
	}()

	// Watch certificate
	return watcher.Add(certificatePath)
}

func fileHash(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	//goland:noinspection GoUnhandledErrorResult
	defer f.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return nil, err
	}

	return hash.Sum(nil), nil
}
