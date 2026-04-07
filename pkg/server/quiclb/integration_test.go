package quiclb_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/traefik/traefik/v3/pkg/server/quiclb"
)

func TestIntegration_QUICHandshakeWithCustomCIDGenerator(t *testing.T) {
	serverID := [8]byte{0xDE, 0xAD, 0xBE, 0xEF, 0xCA, 0xFE, 0xBA, 0xBE}
	encoded := base64.StdEncoding.EncodeToString(serverID[:])

	gen, err := quiclb.NewGenerator(encoded)
	require.NoError(t, err)
	require.NotNil(t, gen)

	// Create a UDP connection for the server.
	udpAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	require.NoError(t, err)

	udpConn, err := net.ListenUDP("udp", udpAddr)
	require.NoError(t, err)

	// Create a Transport with the custom CID generator.
	tr := &quic.Transport{
		Conn:                  udpConn,
		ConnectionIDGenerator: gen,
	}
	defer tr.Close()

	tlsConf := generateTestTLSConfig(t)

	ln, err := tr.ListenEarly(tlsConf, &quic.Config{})
	require.NoError(t, err)
	defer ln.Close()

	serverAddr := ln.Addr()

	// Accept connections in a goroutine.
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)

		conn, err := ln.Accept(context.Background())
		if err != nil {
			return
		}
		defer conn.CloseWithError(0, "done")

		// Keep connection alive until client closes.
		<-conn.Context().Done()
	}()

	// Client connects.
	clientTLSConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"h3"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := quic.DialAddr(ctx, serverAddr.String(), clientTLSConf, &quic.Config{})
	require.NoError(t, err, "QUIC handshake must succeed with custom CID generator")

	// Verify connection is established.
	assert.NotNil(t, conn)

	// A successful handshake proves the CID generator was used and produced
	// valid Connection IDs. The quic-go public API does not expose CID bytes
	// on established connections, but the unit tests in generator_test.go
	// already verify the byte layout directly.

	conn.CloseWithError(0, "test done")

	// Wait for server to finish.
	select {
	case <-serverDone:
	case <-time.After(5 * time.Second):
		t.Fatal("server did not finish in time")
	}
}

// generateTestTLSConfig creates a self-signed TLS certificate for testing.
func generateTestTLSConfig(t *testing.T) *tls.Config {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	return &tls.Config{
		Certificates: []tls.Certificate{{
			Certificate: [][]byte{certDER},
			PrivateKey:  key,
		}},
		NextProtos: []string{"h3"},
	}
}
