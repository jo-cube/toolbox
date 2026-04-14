package ksetoff

import (
	"bufio"
	"fmt"
	"os"
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
			return nil, fmt.Errorf("config file %s:%d: expected key=value, got %q", path, lineNo, line)
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			return nil, fmt.Errorf("config file %s:%d: empty key", path, lineNo)
		}

		switch key {
		case "bootstrap.servers", "metadata.broker.list":
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
			cfg.SSLVerify = strings.ToLower(value) != "false"
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("config file %s: bootstrap.servers is required", path)
	}

	return cfg, nil
}
