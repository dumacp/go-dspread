package main

import (
	"encoding/hex"
	"fmt"
)

func parseHostKey(value string) ([]byte, error) {
	if value == "" {
		return nil, nil
	}
	key, err := hex.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("invalid SAM host key encoding: %w", err)
	}
	if len(key) != 16 {
		return nil, fmt.Errorf("SAM host key must contain exactly 16 bytes")
	}
	return key, nil
}
