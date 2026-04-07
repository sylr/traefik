package quiclb

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateConnectionID_Layout(t *testing.T) {
	serverID := [serverIDLen]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	encoded := base64.StdEncoding.EncodeToString(serverID[:])

	gen, err := NewGenerator(encoded)
	require.NoError(t, err)
	require.NotNil(t, gen)

	cid, err := gen.GenerateConnectionID()
	require.NoError(t, err)

	cidBytes := cid.Bytes()

	// CID must be 16 bytes.
	assert.Len(t, cidBytes, cidLen)

	// Header: top 3 bits must be zero (config_rotation = 0).
	assert.Zero(t, cidBytes[0]&0xE0, "config_rotation bits must be zero")

	// Server ID at bytes [1:9].
	assert.Equal(t, serverID[:], cidBytes[1:1+serverIDLen])
}

func TestGenerateConnectionID_UniqueNonce(t *testing.T) {
	serverID := [serverIDLen]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF, 0x11, 0x22}
	encoded := base64.StdEncoding.EncodeToString(serverID[:])

	gen, err := NewGenerator(encoded)
	require.NoError(t, err)

	cid1, err := gen.GenerateConnectionID()
	require.NoError(t, err)

	cid2, err := gen.GenerateConnectionID()
	require.NoError(t, err)

	// Two CIDs must differ (nonce is random).
	assert.NotEqual(t, cid1.Bytes(), cid2.Bytes())

	// But the server ID portion must be the same.
	assert.Equal(t, cid1.Bytes()[1:1+serverIDLen], cid2.Bytes()[1:1+serverIDLen])
}

func TestConnectionIDLen(t *testing.T) {
	serverID := [serverIDLen]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	encoded := base64.StdEncoding.EncodeToString(serverID[:])

	gen, err := NewGenerator(encoded)
	require.NoError(t, err)

	assert.Equal(t, cidLen, gen.ConnectionIDLen())
}

func TestNewGenerator_Valid(t *testing.T) {
	serverID := [serverIDLen]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	encoded := base64.StdEncoding.EncodeToString(serverID[:])

	gen, err := NewGenerator(encoded)
	require.NoError(t, err)
	require.NotNil(t, gen)
	assert.Equal(t, serverID, gen.serverID)
}

func TestNewGenerator_Empty(t *testing.T) {
	gen, err := NewGenerator("")
	assert.NoError(t, err)
	assert.Nil(t, gen)
}

func TestNewGenerator_InvalidBase64(t *testing.T) {
	gen, err := NewGenerator("not-valid-base64!!!")
	assert.Error(t, err)
	assert.Nil(t, gen)
	assert.Contains(t, err.Error(), "decoding server ID")
}

func TestNewGenerator_WrongLength(t *testing.T) {
	// 4 bytes instead of 8.
	short := base64.StdEncoding.EncodeToString([]byte{0x01, 0x02, 0x03, 0x04})

	gen, err := NewGenerator(short)
	assert.Error(t, err)
	assert.Nil(t, gen)
	assert.Contains(t, err.Error(), "server ID must be 8 bytes, got 4")

	// 12 bytes instead of 8.
	long := base64.StdEncoding.EncodeToString([]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C})

	gen, err = NewGenerator(long)
	assert.Error(t, err)
	assert.Nil(t, gen)
	assert.Contains(t, err.Error(), "server ID must be 8 bytes, got 12")
}
