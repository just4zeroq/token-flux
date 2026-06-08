// Package attestation provides binary integrity verification.
// Node reads its own executable, extracts random byte segments
// per platform challenge, and computes an HMAC proof.
package attestation

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
)

// Challenge is issued by the platform to verify binary integrity.
type Challenge struct {
	Version  string   `json:"version"`
	Nonce    string   `json:"nonce"`    // hex-encoded 32-byte random nonce
	Offsets  [][2]int `json:"offsets"`  // [[start_byte, length], ...]
}

// Proof is the node's response to an attestation Challenge.
type Proof struct {
	HMAC string `json:"hmac"` // hex(HMAC-SHA256(nonce, concatenated_segments))
}

// ComputeProof reads the binary at exePath, extracts byte segments
// at the specified offsets, and computes HMAC-SHA256(nonce, segments).
func ComputeProof(exePath string, c *Challenge) (*Proof, error) {
	data, err := os.ReadFile(exePath)
	if err != nil {
		return nil, fmt.Errorf("read binary: %w", err)
	}

	nonce, err := hex.DecodeString(c.Nonce)
	if err != nil {
		return nil, fmt.Errorf("decode nonce: %w", err)
	}

	var buf []byte
	for i, off := range c.Offsets {
		start := off[0]
		length := off[1]
		if start < 0 || length <= 0 {
			return nil, fmt.Errorf("invalid offset %d: [%d, %d]", i, start, length)
		}
		if start+length > len(data) {
			return nil, fmt.Errorf("offset %d+%d exceeds binary size %d", start, length, len(data))
		}
		buf = append(buf, data[start:start+length]...)
	}

	mac := hmac.New(sha256.New, nonce)
	mac.Write(buf)
	hmacBytes := mac.Sum(nil)

	return &Proof{HMAC: hex.EncodeToString(hmacBytes)}, nil
}

// SelfSize returns the size of the binary at exePath.
func SelfSize(exePath string) (int64, error) {
	fi, err := os.Stat(exePath)
	if err != nil {
		return 0, fmt.Errorf("stat binary: %w", err)
	}
	return fi.Size(), nil
}

// SelfSHA256 returns the SHA256 of the binary at exePath.
func SelfSHA256(exePath string) (string, error) {
	data, err := os.ReadFile(exePath)
	if err != nil {
		return "", fmt.Errorf("read binary: %w", err)
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:]), nil
}
