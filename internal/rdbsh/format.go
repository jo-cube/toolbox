package rdbsh

import (
	"encoding/hex"
	"fmt"
	"strings"
)

func isPrintable(data []byte) bool {
	if len(data) == 0 {
		return true
	}
	for _, b := range data {
		if b >= 0x20 && b <= 0x7E {
			continue
		}
		if b == '\n' || b == '\r' || b == '\t' {
			continue
		}
		return false
	}
	return true
}

func formatBytes(data []byte) string {
	if len(data) == 0 {
		return "(empty)"
	}
	if isPrintable(data) {
		return string(data)
	}
	return fmt.Sprintf("%x", data)
}

func parseInput(value string) ([]byte, error) {
	if strings.HasPrefix(value, "0x") || strings.HasPrefix(value, "0X") {
		decoded, err := hex.DecodeString(value[2:])
		if err != nil {
			return nil, fmt.Errorf("invalid hex input %q: %w", value, err)
		}
		return decoded, nil
	}
	return []byte(value), nil
}
