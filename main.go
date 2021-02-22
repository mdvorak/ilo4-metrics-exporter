package main

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/mdvorak/ilo4-metrics-exporter/pkg/ilo4"
	"github.com/namsral/flag"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

func newZapLogger(production bool) (*zap.Logger, error) {
	if production {
		return zap.NewProduction()
	}
	return zap.NewDevelopment()
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
	httpClient, err := newHTTPClient(log, certificatePath)
	if err != nil {
		panic(err)
	}

	// Credentials
	credentials, err := readCredentials(credentialsPath)
	if err != nil {
		log.Error(err, "failed to read credentials")
	}

	// Client
	log.Info("initializing iLO4 client", "url", url)
	iloClient := ilo4.NewClient(log.WithName("ilo4-client"), httpClient, url, credentials)

	// Watch certificates and credentials
	err = watchConfigurationChanges(log, iloClient, certificatePath, credentialsPath)
	if err != nil {
		log.Error(err, "failed to setup filesystem watcher, certificate updates won't be available")
	} else {
		log.Info("watching certificate for changes", "path", certificatePath)
		log.Info("watching credentials for changes", "path", credentialsPath)
	}

	// Metrics
	prometheus.MustRegister(ilo4.NewMetrics(iloClient))

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

func newHTTPClient(log logr.Logger, certificatePath string) (*http.Client, error) {
	// TLS
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

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

	// Create client
	client := &http.Client{
		Transport: &http.Transport{TLSClientConfig: tlsConfig},
	}

	// Success
	return client, nil
}

func watchConfigurationChanges(log logr.Logger, iloClient *ilo4.Client, certificatePath string, credentialsPath string) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	lastCertificateHash, err := fileHash(certificatePath)
	if err != nil {
		log.V(1).Error(err, "failed get certificate file hash", "path", certificatePath)
	}

	// Async certificate updates
	go func() {
		for {
			select {
			case evt := <-watcher.Events:
				if evt.Name == certificatePath {
					lastCertificateHash = certificateFileChanged(log, iloClient, certificatePath, lastCertificateHash)
				} else if evt.Name == credentialsPath {
					credentialsFileChanged(log, iloClient, credentialsPath)
				}

			case err := <-watcher.Errors:
				log.Error(err, "filesystem watcher failed")
			}
		}
	}()

	// Watch certificate
	if err := watcher.Add(certificatePath); err != nil {
		return fmt.Errorf("failed to add path %s to watcher: %w", certificatePath, err)
	}

	// Watch credentials
	if err := watcher.Add(credentialsPath); err != nil {
		return fmt.Errorf("failed to add path %s to watcher: %w", credentialsPath, err)
	}

	// Success
	return nil
}

func certificateFileChanged(log logr.Logger, iloClient *ilo4.Client, certificatePath string, lastHash []byte) (hash []byte) {
	// Get current checksum
	hash, err := fileHash(certificatePath)
	if err != nil {
		log.V(1).Error(err, "failed get certificate file hash", "path", certificatePath)
	}

	// Only if hash changed
	if !bytes.Equal(hash, lastHash) || hash == nil {
		log.Info("server certificate changed")
		if httpClient, err := newHTTPClient(log, certificatePath); err != nil {
			log.Error(err, "failed to replace http client with new certificate")
		} else {
			// Replace client with new certificate
			iloClient.Client = httpClient
		}
	}

	// Return hash
	return
}

func credentialsFileChanged(log logr.Logger, iloClient *ilo4.Client, credentialsPath string) {
	log.Info("server credentials changed")
	credentials, err := readCredentials(credentialsPath)
	if err != nil {
		log.Error(err, "failed to read updated credentials")
	} else {
		iloClient.Credentials = credentials
	}
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

func readCredentials(path string) (ilo4.Credentials, error) {
	// Open
	f, err := os.Open(path)
	if err != nil {
		return ilo4.Credentials{}, fmt.Errorf("failed to open credentials file %s: %w", path, err)
	}

	//goland:noinspection GoUnhandledErrorResult
	defer f.Close()

	// Read
	var result ilo4.Credentials
	if err := json.NewDecoder(f).Decode(&result); err != nil {
		return ilo4.Credentials{}, fmt.Errorf("failed to deserialize credentials json: %w", err)
	}

	// Success
	return result, nil
}
