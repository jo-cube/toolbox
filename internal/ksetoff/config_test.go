package ksetoff

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseConfigFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    *KafkaConfig
		wantErr string
	}{
		{
			name:    "minimal config",
			content: "bootstrap.servers=broker1:9092, broker2:9092\n",
			want:    &KafkaConfig{Brokers: []string{"broker1:9092", "broker2:9092"}, SecurityProtocol: "PLAINTEXT", SSLVerify: true},
		},
		{
			name: "full config",
			content: strings.Join([]string{
				"# comment",
				"bootstrap.servers=broker1:9092",
				"security.protocol=sasl_ssl",
				"sasl.mechanism=scram-sha-512",
				"sasl.username=user",
				"sasl.password=pass",
				"ssl.ca.location=/tmp/ca.pem",
				"ssl.certificate.location=/tmp/cert.pem",
				"ssl.key.location=/tmp/key.pem",
				"ssl.key.password=secret",
				"enable.ssl.certificate.verification=false",
				"unknown.key=ignored",
			}, "\n"),
			want: &KafkaConfig{
				Brokers:          []string{"broker1:9092"},
				SecurityProtocol: "SASL_SSL",
				SASLMechanism:    "SCRAM-SHA-512",
				SASLUsername:     "user",
				SASLPassword:     "pass",
				SSLCALocation:    "/tmp/ca.pem",
				SSLCertLocation:  "/tmp/cert.pem",
				SSLKeyLocation:   "/tmp/key.pem",
				SSLKeyPassword:   "secret",
				SSLVerify:        false,
			},
		},
		{
			name:    "missing brokers",
			content: "security.protocol=PLAINTEXT\n",
			wantErr: "bootstrap.servers is required",
		},
		{
			name:    "missing equals",
			content: "bootstrap.servers broker1:9092\n",
			wantErr: "expected key=value",
		},
		{
			name:    "empty key",
			content: "=value\nbootstrap.servers=broker1:9092\n",
			wantErr: "empty key",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			path := filepath.Join(dir, "kafka.conf")
			if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}

			got, err := ParseConfigFile(path)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ParseConfigFile() error = %v, want substring %q", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseConfigFile() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ParseConfigFile() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
