package ksetoff

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type KafkaConfig struct {
	Brokers          []string
	SecurityProtocol string
	SASLMechanism    string
	SASLUsername     string
	SASLPassword     string
	SSLCALocation    string
	SSLCertLocation  string
	SSLKeyLocation   string
	SSLKeyPassword   string
	SSLVerify        bool
}

func ParseConfigFile(path string) (*KafkaConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config file: %w", err)
	}
	defer f.Close()

	cfg := &KafkaConfig{
		SecurityProtocol: "PLAINTEXT",
		SSLVerify:        true,
	}

	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, found := strings.Cut(line, "=")
		if !found {
			return nil, fmt.Errorf("config file %s:%d: expected key=value", path, lineNo)
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			return nil, fmt.Errorf("config file %s:%d: empty key", path, lineNo)
		}

		switch key {
		case "bootstrap.servers", "metadata.broker.list":
			cfg.Brokers = cfg.Brokers[:0]
			for _, broker := range strings.Split(value, ",") {
				broker = strings.TrimSpace(broker)
				if broker != "" {
					cfg.Brokers = append(cfg.Brokers, broker)
				}
			}
		case "security.protocol":
			cfg.SecurityProtocol = strings.ToUpper(value)
		case "sasl.mechanism", "sasl.mechanisms":
			cfg.SASLMechanism = strings.ToUpper(value)
		case "sasl.username":
			cfg.SASLUsername = value
		case "sasl.password":
			cfg.SASLPassword = value
		case "ssl.ca.location":
			cfg.SSLCALocation = value
		case "ssl.certificate.location":
			cfg.SSLCertLocation = value
		case "ssl.key.location":
			cfg.SSLKeyLocation = value
		case "ssl.key.password":
			cfg.SSLKeyPassword = value
		case "enable.ssl.certificate.verification":
			verify, err := strconv.ParseBool(value)
			if err != nil {
				return nil, fmt.Errorf("config file %s:%d: enable.ssl.certificate.verification must be a valid boolean", path, lineNo)
			}
			cfg.SSLVerify = verify
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("config file %s: bootstrap.servers is required", path)
	}
	if !validSecurityProtocol(cfg.SecurityProtocol) {
		return nil, fmt.Errorf("config file %s: unsupported security.protocol %q (supported: PLAINTEXT, SSL, SASL_PLAINTEXT, SASL_SSL)", path, cfg.SecurityProtocol)
	}

	return cfg, nil
}

func validSecurityProtocol(protocol string) bool {
	switch protocol {
	case "PLAINTEXT", "SSL", "SASL_PLAINTEXT", "SASL_SSL":
		return true
	default:
		return false
	}
}
