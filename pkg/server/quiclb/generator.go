// Package quiclb implements QUIC Connection ID generation compatible with
// IETF draft-ietf-quic-load-balancers (unencrypted mode) for AWS NLB.
//
// When deployed behind an AWS Network Load Balancer with QUIC passthrough,
// the NLB routes packets by reading a Server ID embedded in the QUIC
// Connection ID. This package generates CIDs that contain the Server ID
// provided by the AWS Load Balancer Controller via the AWS_LBC_QUIC_SERVER_ID
// environment variable.
//
// CID byte layout (IETF QUIC-LB unencrypted mode):
//
//	Offset  Length  Field
//	0       1       Header: [3 bits config_rotation=0][5 bits random]
//	1       8       Server ID (from AWS_LBC_QUIC_SERVER_ID, base64-decoded)
//	9       7       Nonce (crypto/rand)
//	Total: 16 bytes
package quiclb

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/quic-go/quic-go"
)

const (
	serverIDLen = 8
	nonceLen    = 7
	cidLen      = 1 + serverIDLen + nonceLen // 16
)

// Generator implements quic.ConnectionIDGenerator with QUIC-LB unencrypted
// mode CIDs containing an AWS NLB Server ID.
//
// Generator is safe for concurrent use.
type Generator struct {
	serverID [serverIDLen]byte
}

// NewGenerator creates a Generator from a base64-encoded server ID string.
// If serverIDBase64 is empty, it returns (nil, nil) indicating no custom
// generator is needed. If the value is invalid, it returns an error.
func NewGenerator(serverIDBase64 string) (*Generator, error) {
	if serverIDBase64 == "" {
		return nil, nil
	}

	decoded, err := base64.StdEncoding.DecodeString(serverIDBase64)
	if err != nil {
		return nil, fmt.Errorf("decoding server ID: %w", err)
	}

	if len(decoded) != serverIDLen {
		return nil, fmt.Errorf("server ID must be %d bytes, got %d", serverIDLen, len(decoded))
	}

	g := &Generator{}
	copy(g.serverID[:], decoded)

	return g, nil
}

// GenerateConnectionID produces a 16-byte QUIC Connection ID with the
// server ID embedded per IETF QUIC-LB unencrypted mode.
func (g *Generator) GenerateConnectionID() (quic.ConnectionID, error) {
	var buf [cidLen]byte

	// Fill entire buffer with random bytes first (header + nonce).
	if _, err := rand.Read(buf[:]); err != nil {
		return quic.ConnectionID{}, fmt.Errorf("generating connection ID: %w", err)
	}

	// Header: zero the top 3 bits (config_rotation = 0).
	buf[0] &= 0x1F

	// Server ID at bytes [1:9].
	copy(buf[1:1+serverIDLen], g.serverID[:])

	// Nonce at bytes [9:16] is already random from the initial rand.Read.

	return quic.ConnectionIDFromBytes(buf[:]), nil
}

// ConnectionIDLen returns the constant length of generated Connection IDs.
func (g *Generator) ConnectionIDLen() int {
	return cidLen
}
