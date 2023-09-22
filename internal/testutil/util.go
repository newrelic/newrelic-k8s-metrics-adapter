// Copyright 2022 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package testutil provides common helper for tests.
package testutil

// TODO: replace io/ioutil, k8s.io/utils/pointer - it was deprecated
import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

const (
	// KubeconfigEnv is a environment variable name which points to kubeconfig file which will be used for testing.
	KubeconfigEnv = "KUBECONFIG"

	// Arbitrary amount of time to let tests exit cleanly before main process terminates.
	timeoutGracePeriod   = 10 * time.Second
	certValidityDuration = 1 * time.Hour
	testHost             = "127.0.0.1"
	testKeyRSABits       = 2048
	testFileMode         = 0o600
)

// ContextWithDeadline returns context with will timeout before t.Deadline().
func ContextWithDeadline(t *testing.T) context.Context {
	t.Helper()

	deadline, ok := t.Deadline()
	if !ok {
		return context.Background()
	}

	ctx, cancel := context.WithDeadline(context.Background(), deadline.Truncate(timeoutGracePeriod))

	t.Cleanup(cancel)

	return ctx
}

// TestEnv is a helper struct for accessing data required for integration and e2e tests like API key
// and to perform common test tasks, like generating clients, parameters for adapter etc.
type TestEnv struct {
	PersonalAPIKey  string
	AccountID       string
	Region          string
	Config          string
	ConfigPath      string
	ServingCertPath string
	ServingKeyPath  string
	KubeconfigPath  string
	Port            string
	Host            string
	BaseURL         string
	Flags           []string
	ContextTimeout  time.Duration
	StartKubernetes bool
	Context         context.Context
	RestConfig      *rest.Config
	HTTPClient      *http.Client
}

// Generate fills empty TestEnv struct fields with values.
//
//nolint:funlen,cyclop,nestif
func (te *TestEnv) Generate(t *testing.T) {
	t.Helper()

	if te == nil {
		t.Fatalf("Got nil testing environment pointer")
	}

	if te.ConfigPath == "" {
		te.ConfigPath = filepath.Join(t.TempDir(), "config.yaml")
	}

	if te.PersonalAPIKey == "" {
		te.PersonalAPIKey = os.Getenv("NEWRELIC_API_KEY")
	}

	if te.AccountID == "" {
		te.AccountID = os.Getenv("NEWRELIC_ACCOUNT_ID")
	}

	if te.Region == "" {
		te.Region = os.Getenv("NEWRELIC_REGION")
	}

	if te.Config == "" {
		te.Config = fmt.Sprintf(`
accountID: %s
region: %s
externalMetrics:
  foo:
    query: "SELECT latest(attributeName) FROM (SELECT 0.123 AS 'attributeName' FROM NrUsage)"
    removeClusterFilter: true
  `, te.AccountID, te.Region)
	}

	if err := os.WriteFile(te.ConfigPath, []byte(te.Config), testFileMode); err != nil {
		t.Fatalf("Error writing test config file: %v", err)
	}

	certPath, keyPath, caCert := servingCertsWithCA(t)
	if te.ServingCertPath == "" {
		te.ServingCertPath = certPath
	}

	if te.ServingKeyPath == "" {
		te.ServingKeyPath = keyPath
	}

	if te.KubeconfigPath == "" {
		te.KubeconfigPath = os.Getenv(KubeconfigEnv)
	}

	if te.Context == nil {
		te.Context = ContextWithDeadline(t)
	}

	if te.ContextTimeout != 0 {
		var cancel func()
		te.Context, cancel = context.WithTimeout(ContextWithDeadline(t), te.ContextTimeout)

		t.Cleanup(cancel)
	}

	if te.StartKubernetes {
		testEnv := &envtest.Environment{
			UseExistingCluster: ptr.To(true),
		}

		restConfig, err := testEnv.Start()
		if err != nil {
			t.Fatalf("Starting test environment: %v", err)
		}

		// Append generated serving CA certificate to rest config, so when we create an authenticated
		// HTTP client, it can pass TLS certificate validation
		restConfig.TLSClientConfig.CAData = append(restConfig.TLSClientConfig.CAData, caCert...)

		if te.RestConfig == nil {
			te.RestConfig = restConfig
		}

		tr, err := rest.TransportFor(restConfig)
		if err != nil {
			t.Fatalf("Creating HTTP transport config from kubeconfig: %s", err)
		}

		if te.HTTPClient == nil {
			te.HTTPClient = &http.Client{Transport: tr}
		}

		t.Cleanup(func() {
			if err := testEnv.Stop(); err != nil {
				t.Logf("Stopping test environment: %v", err)
			}
		})
	}

	if te.Port == "" {
		te.Port = fmt.Sprintf("%d", randomUnprivilegedPort(t))
	}

	if te.Host == "" {
		te.Host = testHost
	}

	if te.BaseURL == "" {
		te.BaseURL = fmt.Sprintf("https://%s:%s", te.Host, te.Port)
	}

	if len(te.Flags) == 0 {
		te.Flags = []string{
			"--authentication-kubeconfig=" + te.KubeconfigPath,
			"--authorization-kubeconfig=" + te.KubeconfigPath,
			"--tls-cert-file=" + te.ServingCertPath,
			"--tls-private-key-file=" + te.ServingKeyPath,
			"--secure-port=" + te.Port,
		}
	}
}

// RetryGetRequestAndCheckStatus sends GET request to given URL.
//
// If 401 or 403 return code is received, function will retry.
//
// If other return code is received, function will check the failCondition to fail the given test.
func RetryGetRequestAndCheckStatus(
	ctx context.Context, t *testing.T, httpClient *http.Client, url string, failCondition func(statusCode int) bool,
) []byte {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		t.Fatalf("Creating request: %v", err)
	}

	req.Header = http.Header{"Content-Type": []string{"application/json"}}

	body := []byte{}

	retryUntilFinished(func() bool {
		resp, err := httpClient.Do(req) //nolint:bodyclose // Done via closeResponseBody().

		defer closeResponseBody(t, resp)

		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				t.Fatalf("Test timed out: %v", err)
			}

			t.Logf("Fetching %s: %v", url, err)

			time.Sleep(1 * time.Second)

			return false
		}

		switch {
		// Generic API server does not wait for RequestHeaderAuthRequestController informers cache to be synchronized,
		// so until this is done, metrics adapter will be responding with HTTP 403, so we want to retry on that.
		case resp.StatusCode == http.StatusForbidden, resp.StatusCode == http.StatusUnauthorized:
			t.Logf("Got %d response code, expected %d: %v. Retrying.", resp.StatusCode, http.StatusOK, resp)

			return false
		case failCondition(resp.StatusCode):
			t.Fatalf("Unexpected response code %d", resp.StatusCode)
		}

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Reading response body: %v", err)
		}

		body = data

		return true
	})

	return body
}

// CheckStatusCodeOK calls RetryGetRequestAndCheckStatus with a failCondition function checking for any response not
// equal to 200.
func CheckStatusCodeOK(ctx context.Context, t *testing.T, httpClient *http.Client, url string) []byte {
	t.Helper()

	return RetryGetRequestAndCheckStatus(
		ctx, t, httpClient, url,
		func(statusCode int) bool {
			return statusCode != http.StatusOK
		},
	)
}

// randomUnprivilegedPort returns valid unprivileged random port number which can be used for testing.
func randomUnprivilegedPort(t *testing.T) int {
	t.Helper()

	min := 1024
	max := 65535

	i, err := rand.Int(rand.Reader, big.NewInt(int64(max-min+1)))
	if err != nil {
		t.Fatalf("Generating random port: %v", err)
	}

	return int(i.Int64()) + min
}

// servingCertsWithCA generated paths to serving certificate and private key and returns CA certificate,
// which has been used to sign this certificate.
//
//nolint:funlen // Lengthy helper function.
func servingCertsWithCA(t *testing.T) (string, string, []byte) {
	t.Helper()

	dir := t.TempDir()

	// Generate RSA private key.
	priv, err := rsa.GenerateKey(rand.Reader, testKeyRSABits)
	if err != nil {
		t.Fatalf("Generating RSA key: %v", err)
	}

	// Generate serial number for X.509 certificate.
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 1)

	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		t.Fatalf("Generating serial number: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"example"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(certValidityDuration),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		IPAddresses:           []net.IP{net.ParseIP(testHost)},
	}

	// Create X.509 certificate in DER format.
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("Creating X.509 certificate: %v", err)
	}

	// Encode X.509 certificate into PEM format.
	var cert bytes.Buffer
	if err := pem.Encode(&cert, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
		t.Fatalf("Encoding X.509 certificate into PEM format: %v", err)
	}

	// Convert RSA private key into PKCS8 DER format.
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("Encoding RSA private key into PKCS8 DER format: %v", err)
	}

	// Convert private key from PKCS8 DER format to PEM format.
	var key bytes.Buffer
	if err := pem.Encode(&key, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		t.Fatalf("Encoding RSA private key into PEM format: %v", err)
	}

	keyPath := filepath.Join(dir, "tls.key")
	if err := os.WriteFile(keyPath, key.Bytes(), testFileMode); err != nil {
		t.Fatalf("Writing private key to %q: %v", keyPath, err)
	}

	certPath := filepath.Join(dir, "tls.crt")
	if err := os.WriteFile(certPath, cert.Bytes(), testFileMode); err != nil {
		t.Fatalf("Writing certificate to %q: %v", certPath, err)
	}

	return certPath, keyPath, cert.Bytes()
}

// retryUntilFinished calls given function until it returns true.
func retryUntilFinished(f func() bool) {
	for {
		if f() {
			break
		}
	}
}

// closeResponseBody closes given response's body.
func closeResponseBody(t *testing.T, resp *http.Response) {
	t.Helper()

	if resp == nil || resp.Body == nil {
		return
	}

	if err := resp.Body.Close(); err != nil {
		t.Logf("Closing response body: %v", err)
	}
}
