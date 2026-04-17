package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestIsExpiredCertValidityError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "wrapped expired certificate",
			err: &url.Error{
				Op:  "Get",
				URL: "https://example.com/schema.xsd",
				Err: x509.CertificateInvalidError{Reason: x509.Expired},
			},
			want: true,
		},
		{
			name: "wrapped hostname mismatch",
			err: &url.Error{
				Op:  "Get",
				URL: "https://example.com/schema.xsd",
				Err: x509.HostnameError{},
			},
			want: false,
		},
		{
			name: "wrapped other certificate invalid reason",
			err: &url.Error{
				Op:  "Get",
				URL: "https://example.com/schema.xsd",
				Err: x509.CertificateInvalidError{Reason: x509.NotAuthorizedToSign},
			},
			want: false,
		},
		{
			name: "non certificate error",
			err:  errors.New("dial tcp timeout"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isExpiredCertValidityError(tt.err); got != tt.want {
				t.Fatalf("isExpiredCertValidityError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestInsecureTLSClientClonesTimeoutAndSkipsVerify(t *testing.T) {
	t.Parallel()

	original := &http.Client{Timeout: 37 * time.Second}
	retry := insecureTLSClient(original)

	if retry == original {
		t.Fatal("insecureTLSClient returned the original client")
	}
	if retry.Timeout != original.Timeout {
		t.Fatalf("retry timeout = %v, want %v", retry.Timeout, original.Timeout)
	}

	transport, ok := retry.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("retry transport type = %T, want *http.Transport", retry.Transport)
	}
	if transport.TLSClientConfig == nil {
		t.Fatal("retry transport missing TLS config")
	}
	if !transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("retry transport should skip certificate verification")
	}

	originalTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		t.Fatal("default transport is not *http.Transport")
	}
	if originalTransport.TLSClientConfig != nil && originalTransport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("default transport should not be mutated")
	}

	// Keep the import of crypto/tls meaningful and guard the actual config type.
	var _ *tls.Config = transport.TLSClientConfig
}
