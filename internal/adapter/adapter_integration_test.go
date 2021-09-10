// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package adapter_test

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
	"io/ioutil"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/gsanchezgavier/metrics-adapter/internal/adapter"
	"github.com/gsanchezgavier/metrics-adapter/internal/provider/mock"
)

const (
	// Arbitrary amount of time to let tests exit cleanly before main process terminates.
	timeoutGracePeriod   = 10 * time.Second
	certValidityDuration = 1 * time.Hour
	testHost             = "127.0.0.1"
	kubeconfigEnv        = "KUBECONFIG"
)

func Test_Adapter_responds_to(t *testing.T) {
	t.Parallel()

	securePort := fmt.Sprintf("%d", randomUnprivilegedPort(t))

	options := adapter.Options{
		Args: []string{
			"--secure-port=" + securePort,
			"--v=2",
		},
		ExternalMetricsProvider: &mock.Provider{},
	}

	ctx, restConfig := runAdapter(t, options)

	httpClient := authorizedHTTPClient(t, restConfig)

	cases := map[string]string{
		"openAPI":        "/openapi/v2",
		"metric_request": "/apis/external.metrics.k8s.io/v1beta1",
	}

	for name, uri := range cases { //nolint:paralleltest // False positive.
		uri := uri

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			url := fmt.Sprintf("https://%s:%s%s", testHost, securePort, uri)

			checkStatusCodeOK(ctx, t, httpClient, url)
		})
	}
}

func checkStatusCodeOK(ctx context.Context, t *testing.T, httpClient http.Client, url string) {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		t.Fatalf("Creating request: %v", err)
	}

	req.Header = http.Header{"Content-Type": []string{"application/json"}}

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

		switch resp.StatusCode {
		// Generic API server does not wait for RequestHeaderAuthRequestController informers cache to be synchronized,
		// so until this is done, metrics adapter will be responding with HTTP 403, so we want to retry on that.
		case http.StatusForbidden, http.StatusUnauthorized:
			t.Logf("Got %d response code, expected %d: %v. Retrying.", resp.StatusCode, http.StatusOK, resp)

			return false
		case http.StatusOK:
			return true
		default:
			t.Fatalf("Got %d response code, expected %d: %v", resp.StatusCode, http.StatusOK, resp)
		}

		return false
	})
}

func runAdapter(t *testing.T, options adapter.Options) (context.Context, *rest.Config) {
	t.Helper()

	ctxWithDeadline := contextWithDeadline(t)

	ctx, cancel := context.WithCancel(ctxWithDeadline)

	useExistingCluster := true
	testEnv := &envtest.Environment{
		UseExistingCluster: &useExistingCluster,
	}

	certPath, keyPath, caCert := testServingCertsWithCA(t)

	restConfig, err := testEnv.Start()
	if err != nil {
		t.Fatalf("Starting test environment: %v", err)
	}

	// Append generated serving CA certificate to rest config, so when we create an authenticated
	// HTTP client, it can pass TLS certificate validation.
	restConfig.TLSClientConfig.CAData = append(restConfig.TLSClientConfig.CAData, caCert...)

	t.Cleanup(func() {
		cancel()
		if err := testEnv.Stop(); err != nil {
			t.Logf("Stopping test environment: %v", err)
		}
	})

	kubeconfig := os.Getenv(kubeconfigEnv)
	args := []string{
		"--authentication-kubeconfig=" + kubeconfig,
		"--authorization-kubeconfig=" + kubeconfig,
		"--tls-cert-file=" + certPath,
		"--tls-private-key-file=" + keyPath,
	}
	options.Args = append(options.Args, args...)

	adapter, err := adapter.NewAdapter(options)
	if err != nil {
		t.Fatalf("Creating adapter: %v", err)
	}

	go func() {
		if err := adapter.Run(ctx.Done()); err != nil {
			t.Logf("Running operator: %v\n", err)
			t.Fail()
		}
	}()

	return ctx, restConfig
}

// contextWithDeadline returns context which will timeout before t.Deadline().
func contextWithDeadline(t *testing.T) context.Context {
	t.Helper()

	deadline, ok := t.Deadline()
	if !ok {
		return context.Background()
	}

	ctx, cancel := context.WithDeadline(context.Background(), deadline.Truncate(timeoutGracePeriod))

	t.Cleanup(cancel)

	return ctx
}

func retryUntilFinished(f func() bool) {
	for {
		if f() {
			break
		}
	}
}

func closeResponseBody(t *testing.T, resp *http.Response) {
	t.Helper()

	if resp == nil || resp.Body == nil {
		return
	}

	if err := resp.Body.Close(); err != nil {
		t.Logf("Closing response body: %v", err)
	}
}

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

// authorizedHTTPClient returns an http client with authentication defined for the rest config.
func authorizedHTTPClient(t *testing.T, restConfig *rest.Config) http.Client {
	t.Helper()

	tr, err := rest.TransportFor(restConfig)
	if err != nil {
		t.Fatalf("Creating HTTP transport config from kubeconfig: %s", err)
	}

	return http.Client{Transport: tr}
}

//nolint:funlen // Lengthy helper function.
func testServingCertsWithCA(t *testing.T) (string, string, []byte) {
	t.Helper()

	dir := t.TempDir()

	// Generate RSA private key.
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Generating RSA key: %v", err)
	}

	// Generate serial number for X.509 certificate.
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)

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
	if err := ioutil.WriteFile(keyPath, key.Bytes(), 0o600); err != nil {
		t.Fatalf("Writing private key to %q: %v", keyPath, err)
	}

	certPath := filepath.Join(dir, "tls.crt")
	if err := ioutil.WriteFile(certPath, cert.Bytes(), 0o600); err != nil {
		t.Fatalf("Writing certificate to %q: %v", certPath, err)
	}

	return certPath, keyPath, cert.Bytes()
}
