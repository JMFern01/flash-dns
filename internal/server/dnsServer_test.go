package server

import (
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

func createDNSQuery(domain string, qtype uint16) []byte {
	var (
		header        []byte = make([]byte, 12)
		question      []byte
		labels        []byte = []byte(domain)
		qtypeBytes    []byte = make([]byte, 2)
		qclassBytes   []byte = make([]byte, 2)
		standardQuery uint16 = 0x0100
		start         int    = 0
		length        int
	)
	binary.BigEndian.PutUint16(header[0:2], 1234)
	binary.BigEndian.PutUint16(header[2:4], standardQuery)
	binary.BigEndian.PutUint16(header[4:6], 1)

	for i := 0; i <= len(labels); i++ {
		if i == len(labels) || labels[i] == '.' {
			length = i - start
			if length > 0 {
				question = append(question, byte(length))
				question = append(question, labels[start:i]...)
			}
			start = i + 1
		}
	}
	question = append(question, 0)

	binary.BigEndian.PutUint16(qtypeBytes, qtype)
	question = append(question, qtypeBytes...)

	binary.BigEndian.PutUint16(qclassBytes, 1)
	question = append(question, qclassBytes...)

	return append(header, question...)
}

func TestNewDNSServer(t *testing.T) {
	var (
		localAddr    string     = "127.0.0.1"
		upstreamDns  string     = "1.1.1.1"
		expectedAddr string     = "127.0.0.1:53"
		expectedDns  string     = "1.1.1.1:53"
		server       *DNSServer = NewDNSServer(localAddr, upstreamDns)
	)

	if server == nil {
		t.Fatal("Server wasn't created")
	}

	if server.cache == nil {
		t.Error("server.cache is nil, should be initialized")
	}

	if expectedAddr != server.localAddr {
		t.Errorf("server.localAddr = %s, expected %s", server.localAddr, expectedAddr)
	}

	if expectedDns != server.upstreamDNS {
		t.Errorf("server.upstreamDNS = %s, expected %s", server.upstreamDNS, expectedDns)
	}
}

func TestContextCancellation(t *testing.T) {
	var (
		server   *DNSServer = NewDNSServer("127.0.0.1", "1.1.1.1")
		ctx      context.Context
		cancel   context.CancelFunc
		query    []byte = createDNSQuery("mock.com", 1)
		udpAddr  *net.UDPAddr
		mockConn *net.UDPConn
		done     chan bool = make(chan bool)
	)
	ctx, cancel = context.WithCancel(context.Background())
	cancel()

	udpAddr, _ = net.ResolveUDPAddr("udp", "127.0.0.1:39181")
	mockConn, _ = net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	defer mockConn.Close()

	go func() {
		server.handleQuery(ctx, query, udpAddr, mockConn)
		done <- true
	}()

	select {
	case <-done:

	case <-time.After(50 * time.Millisecond):
		t.Error("handleContext did not respect context cancellation")
	}
}
