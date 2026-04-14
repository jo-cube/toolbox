package ksetoff

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"
)

func BuildClientOpts(cfg *KafkaConfig) ([]kgo.Opt, error) {
	opts := []kgo.Opt{kgo.SeedBrokers(cfg.Brokers...)}

	needsTLS := cfg.SecurityProtocol == "SSL" || cfg.SecurityProtocol == "SASL_SSL"
	needsSASL := cfg.SecurityProtocol == "SASL_PLAINTEXT" || cfg.SecurityProtocol == "SASL_SSL"

	if needsTLS {
		tlsCfg := &tls.Config{InsecureSkipVerify: !cfg.SSLVerify}

		if cfg.SSLCALocation != "" {
			caCert, err := os.ReadFile(cfg.SSLCALocation)
			if err != nil {
				return nil, fmt.Errorf("read CA cert %s: %w", cfg.SSLCALocation, err)
			}
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(caCert) {
				return nil, fmt.Errorf("failed to parse CA cert from %s", cfg.SSLCALocation)
			}
			tlsCfg.RootCAs = pool
		}

		if cfg.SSLKeyPassword != "" {
			return nil, fmt.Errorf("ssl.key.password is not supported; provide an unencrypted PEM key")
		}

		if cfg.SSLCertLocation != "" && cfg.SSLKeyLocation != "" {
			cert, err := tls.LoadX509KeyPair(cfg.SSLCertLocation, cfg.SSLKeyLocation)
			if err != nil {
				return nil, fmt.Errorf("load client certificate/key (%s, %s): %w", cfg.SSLCertLocation, cfg.SSLKeyLocation, err)
			}
			tlsCfg.Certificates = []tls.Certificate{cert}
		} else if cfg.SSLCertLocation != "" || cfg.SSLKeyLocation != "" {
			return nil, fmt.Errorf("both ssl.certificate.location and ssl.key.location must be set for mTLS (got cert=%q, key=%q)", cfg.SSLCertLocation, cfg.SSLKeyLocation)
		}

		opts = append(opts, kgo.DialTLSConfig(tlsCfg))
	}

	if needsSASL {
		switch cfg.SASLMechanism {
		case "PLAIN":
			mechanism := plain.Auth{User: cfg.SASLUsername, Pass: cfg.SASLPassword}.AsMechanism()
			opts = append(opts, kgo.SASL(mechanism))
		case "SCRAM-SHA-256":
			mechanism := scram.Auth{User: cfg.SASLUsername, Pass: cfg.SASLPassword}.AsSha256Mechanism()
			opts = append(opts, kgo.SASL(mechanism))
		case "SCRAM-SHA-512":
			mechanism := scram.Auth{User: cfg.SASLUsername, Pass: cfg.SASLPassword}.AsSha512Mechanism()
			opts = append(opts, kgo.SASL(mechanism))
		case "":
			return nil, fmt.Errorf("sasl.mechanism is required when security.protocol is %s", cfg.SecurityProtocol)
		default:
			return nil, fmt.Errorf("unsupported sasl.mechanism: %s (supported: PLAIN, SCRAM-SHA-256, SCRAM-SHA-512)", cfg.SASLMechanism)
		}
	}

	opts = append(opts, kgo.DialTimeout(10*time.Second))
	opts = append(opts, kgo.RequestTimeoutOverhead(15*time.Second))

	return opts, nil
}

func NewAdminClient(ctx context.Context, cfg *KafkaConfig) (*kadm.Client, func(), error) {
	opts, err := BuildClientOpts(cfg)
	if err != nil {
		return nil, nil, err
	}

	client, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create kafka client: %w", err)
	}

	cleanup := func() {
		client.Close()
	}

	if err := client.Ping(ctx); err != nil {
		cleanup()
		if netErr, ok := err.(*net.OpError); ok {
			return nil, nil, fmt.Errorf("cannot connect to kafka at %s: %w", netErr.Addr, netErr.Err)
		}
		return nil, nil, fmt.Errorf("cannot connect to kafka brokers (%s): %w", strings.Join(cfg.Brokers, ","), err)
	}

	admin := kadm.NewClient(client)
	return admin, cleanup, nil
}
