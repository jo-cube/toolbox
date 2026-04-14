package ksetoff

import (
	"strings"
	"testing"
)

func TestBuildClientOptsErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     *KafkaConfig
		wantErr string
	}{
		{
			name: "missing sasl mechanism",
			cfg: &KafkaConfig{
				Brokers:          []string{"broker1:9092"},
				SecurityProtocol: "SASL_SSL",
				SSLVerify:        true,
			},
			wantErr: "sasl.mechanism is required",
		},
		{
			name: "unsupported sasl mechanism",
			cfg: &KafkaConfig{
				Brokers:          []string{"broker1:9092"},
				SecurityProtocol: "SASL_PLAINTEXT",
				SASLMechanism:    "GSSAPI",
				SSLVerify:        true,
			},
			wantErr: "unsupported sasl.mechanism",
		},
		{
			name: "encrypted key unsupported",
			cfg: &KafkaConfig{
				Brokers:          []string{"broker1:9092"},
				SecurityProtocol: "SSL",
				SSLKeyPassword:   "secret",
				SSLVerify:        true,
			},
			wantErr: "ssl.key.password is not supported",
		},
		{
			name: "incomplete mtls",
			cfg: &KafkaConfig{
				Brokers:          []string{"broker1:9092"},
				SecurityProtocol: "SSL",
				SSLCertLocation:  "/tmp/client.pem",
				SSLVerify:        true,
			},
			wantErr: "both ssl.certificate.location and ssl.key.location must be set",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := BuildClientOpts(tt.cfg)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("BuildClientOpts() error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}
