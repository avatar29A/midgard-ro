package network

import (
	"encoding/binary"
	"net"
	"os"
	"testing"
	"time"

	"github.com/Faultbox/midgard-ro/internal/logger"
)

// TestMain initialises the logger. The network code logs on its read path and
// zap panics on a nil logger, so without this the tests fail inside logging
// rather than on what they are checking.
func TestMain(m *testing.M) {
	_ = logger.InitWithFileConfig("error", logger.FileConfig{}, false)
	os.Exit(m.Run())
}

// dialLoopback returns a connected client/server pair over real TCP, so the
// read path under test is the same one the game uses.
func dialLoopback(t *testing.T) (client, server net.Conn) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	accepted := make(chan net.Conn, 1)
	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			accepted <- nil
			return
		}
		accepted <- conn
	}()

	client, err = net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	server = <-accepted
	if server == nil {
		t.Fatal("accept failed")
	}
	t.Cleanup(func() { _ = server.Close() })

	return client, server
}

// TestProcessDeliversBufferedPacket is the regression guard for a bug that
// disabled all inbound networking without looking broken.
//
// Process polls the socket with a read deadline. Setting that deadline to
// time.Now() looks like "return whatever is buffered", but Go checks the
// deadline before attempting the syscall, so an expired deadline returns
// ErrDeadlineExceeded having read nothing — buffered data included. The
// connection stays up and only the replies go missing, so the symptom is a
// login button that does nothing rather than an error.
func TestProcessDeliversBufferedPacket(t *testing.T) {
	clientConn, serverConn := dialLoopback(t)

	c := New()
	c.conn = clientConn
	c.connected = true
	c.serverType = ServerMap

	got := make(chan int, 1)
	c.RegisterHandler(0x007F, func(data []byte) error { // ZC_NOTIFY_TIME, 6 bytes
		got <- len(data)
		return nil
	})

	packet := make([]byte, 6)
	binary.LittleEndian.PutUint16(packet[0:2], 0x007F)
	binary.LittleEndian.PutUint32(packet[2:6], 12345)
	if _, err := serverConn.Write(packet); err != nil {
		t.Fatalf("server write: %v", err)
	}

	// Give the bytes time to land in the client's socket buffer, so the read
	// has data waiting rather than needing to block for it.
	time.Sleep(50 * time.Millisecond)

	if err := c.Process(); err != nil {
		t.Fatalf("Process: %v", err)
	}

	select {
	case n := <-got:
		if n != 6 {
			t.Errorf("handler received %d bytes, want 6", n)
		}
	default:
		t.Fatal("handler never fired: Process read nothing despite a packet " +
			"waiting in the socket buffer")
	}
}

// TestProcessReturnsPromptlyWhenIdle checks the other half of the trade-off:
// with no data waiting, Process must not park the game loop. It used to wait
// 10ms here, which cost more per frame than rendering the map.
func TestProcessReturnsPromptlyWhenIdle(t *testing.T) {
	clientConn, _ := dialLoopback(t)

	c := New()
	c.conn = clientConn
	c.connected = true
	c.serverType = ServerMap

	start := time.Now()
	for i := 0; i < 10; i++ {
		if err := c.Process(); err != nil {
			t.Fatalf("Process: %v", err)
		}
	}
	perCall := time.Since(start) / 10

	// A frame is 16.7ms; a tenth of that per poll is the outer limit of
	// tolerable. The old 10ms deadline blew straight through this.
	if perCall > 2*time.Millisecond {
		t.Errorf("idle Process took %v per call, too much of a frame budget", perCall)
	}
}

// TestProcessHandlesSplitPacket covers a packet arriving across two reads: the
// first half must be buffered rather than parsed or discarded.
func TestProcessHandlesSplitPacket(t *testing.T) {
	clientConn, serverConn := dialLoopback(t)

	c := New()
	c.conn = clientConn
	c.connected = true
	c.serverType = ServerMap

	fired := make(chan struct{}, 1)
	c.RegisterHandler(0x007F, func([]byte) error {
		fired <- struct{}{}
		return nil
	})

	packet := make([]byte, 6)
	binary.LittleEndian.PutUint16(packet[0:2], 0x007F)

	if _, err := serverConn.Write(packet[:3]); err != nil {
		t.Fatalf("write first half: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if err := c.Process(); err != nil {
		t.Fatalf("Process: %v", err)
	}

	select {
	case <-fired:
		t.Fatal("handler fired on a partial packet")
	default:
	}

	if _, err := serverConn.Write(packet[3:]); err != nil {
		t.Fatalf("write second half: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if err := c.Process(); err != nil {
		t.Fatalf("Process: %v", err)
	}

	select {
	case <-fired:
	default:
		t.Error("handler never fired after the rest of the packet arrived")
	}
}
