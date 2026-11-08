package satmux

import (
	"encoding/hex"
	"errors"
	"net"
	"net/netip"
	"strconv"
	"strings"
)

// ClientConfig is the carrier configuration that can be validated before
// starting a client. Validation does not resolve DNS or contact the server.
type ClientConfig struct {
	ServerAddress string
	PSK           []byte
	Carriers      int
}

// Validate checks structural configuration errors without exposing the PSK.
// Any non-empty key is accepted for compatibility; use a randomly generated
// 32-byte key in production.
func (c ClientConfig) Validate() error {
	if c.Carriers <= 0 {
		return errors.New("satmux: carrier count must be positive")
	}
	if len(c.PSK) == 0 {
		return errors.New("satmux: pre-shared key must not be empty")
	}
	host, port, err := net.SplitHostPort(c.ServerAddress)
	if err != nil || host == "" || strings.ContainsAny(host, " \t\r\n\x00/\\?#[]") {
		return errors.New("satmux: invalid ground server address")
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		if ip.IsUnspecified() {
			return errors.New("satmux: ground server address must not be unspecified")
		}
	} else if strings.Contains(host, ":") {
		return errors.New("satmux: invalid ground server address")
	}
	if port == "" {
		return errors.New("satmux: invalid ground server port")
	}
	for _, ch := range port {
		if ch < '0' || ch > '9' {
			return errors.New("satmux: invalid ground server port")
		}
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return errors.New("satmux: invalid ground server port")
	}
	return nil
}

// ParsePSKHex decodes a hex PSK. ASCII whitespace is permitted as in Python's
// bytes.fromhex; errors do not include the supplied key.
func ParsePSKHex(value string) ([]byte, error) {
	// Whitespace may occur between bytes, but not between the two nibbles of
	// one byte, matching bytes.fromhex.
	var compact strings.Builder
	for i := 0; i < len(value); i++ {
		switch value[i] {
		case ' ', '\t', '\n', '\r', '\v', '\f':
			if compact.Len()%2 != 0 {
				return nil, errors.New("satmux: invalid or empty hexadecimal pre-shared key")
			}
		default:
			compact.WriteByte(value[i])
		}
	}
	value = compact.String()
	key, err := hex.DecodeString(value)
	if err != nil || len(key) == 0 {
		return nil, errors.New("satmux: invalid or empty hexadecimal pre-shared key")
	}
	return key, nil
}
