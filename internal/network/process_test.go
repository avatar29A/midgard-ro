package network

import (
	"encoding/binary"
	"net"
	"strconv"
	"testing"
	"time"
)

// newTestClient returns a Client connected to a loopback listener, along with
// the server end of that connection.
//
// It goes through Connect rather than assigning the socket directly, so the
// reader goroutine and its lifecycle are part of what is under test.
func newTestClient(t *testing.T) (*Client, net.Conn) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	accepted := make(chan net.Conn, 1)
	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			accepted <- nil
			return
		}
		accepted <- conn
	}()

	host, portStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split addr: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}

	c := New()
	if err := c.Connect(host, port, ServerMap); err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(c.Disconnect)

	server := <-accepted
	if server == nil {
		t.Fatal("accept failed")
	}
	t.Cleanup(func() { _ = server.Close() })

	return c, server
}

func notifyTimePacket() []byte {
	packet := make([]byte, 6) // ZC_NOTIFY_TIME
	binary.LittleEndian.PutUint16(packet[0:2], 0x007F)
	binary.LittleEndian.PutUint32(packet[2:6], 12345)
	return packet
}

// waitForHandler polls Process until the handler fires or the deadline passes.
// The reader runs off-thread, so arrival is not instantaneous.
func waitForHandler(t *testing.T, c *Client, fired <-chan int) (int, bool) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := c.Process(); err != nil {
			t.Fatalf("Process: %v", err)
		}
		select {
		case n := <-fired:
			return n, true
		default:
		}
		time.Sleep(5 * time.Millisecond)
	}
	return 0, false
}

// TestProcessDeliversPacket is the regression guard for inbound networking
// breaking silently. A client that connects but never receives leaves the
// connection up and only the replies missing, so the symptom is a login button
// that does nothing rather than an error — which is exactly how a read that
// quietly returned no data once went unnoticed.
func TestProcessDeliversPacket(t *testing.T) {
	c, server := newTestClient(t)

	fired := make(chan int, 1)
	c.RegisterHandler(0x007F, func(data []byte) error {
		fired <- len(data)
		return nil
	})

	if _, err := server.Write(notifyTimePacket()); err != nil {
		t.Fatalf("server write: %v", err)
	}

	n, ok := waitForHandler(t, c, fired)
	if !ok {
		t.Fatal("handler never fired for a packet the server sent")
	}
	if n != 6 {
		t.Errorf("handler received %d bytes, want 6", n)
	}
}

// TestProcessDoesNotBlockWhenIdle is the other half of the trade-off. Reading
// on the game thread meant choosing between parking the loop and polling it;
// the socket is off-thread now, so an idle Process should cost essentially
// nothing. It once cost 10ms a frame, which alone halved the frame rate.
func TestProcessDoesNotBlockWhenIdle(t *testing.T) {
	c, _ := newTestClient(t)

	start := time.Now()
	for i := 0; i < 100; i++ {
		if err := c.Process(); err != nil {
			t.Fatalf("Process: %v", err)
		}
	}
	perCall := time.Since(start) / 100

	if perCall > 100*time.Microsecond {
		t.Errorf("idle Process took %v per call; it should not touch the socket at all", perCall)
	}
}

// TestProcessHandlesSplitPacket covers a packet arriving across two reads: the
// first half must be buffered rather than parsed or discarded.
func TestProcessHandlesSplitPacket(t *testing.T) {
	c, server := newTestClient(t)

	fired := make(chan int, 1)
	c.RegisterHandler(0x007F, func(data []byte) error {
		fired <- len(data)
		return nil
	})

	packet := notifyTimePacket()

	if _, err := server.Write(packet[:3]); err != nil {
		t.Fatalf("write first half: %v", err)
	}

	// Give the half a chance to arrive and be (correctly) held back.
	for i := 0; i < 20; i++ {
		if err := c.Process(); err != nil {
			t.Fatalf("Process: %v", err)
		}
		time.Sleep(5 * time.Millisecond)
	}
	select {
	case <-fired:
		t.Fatal("handler fired on a partial packet")
	default:
	}

	if _, err := server.Write(packet[3:]); err != nil {
		t.Fatalf("write second half: %v", err)
	}
	if _, ok := waitForHandler(t, c, fired); !ok {
		t.Error("handler never fired after the rest of the packet arrived")
	}
}

// TestProcessDeliversBatchedPackets covers several packets arriving in one
// segment, which is how they usually turn up.
func TestProcessDeliversBatchedPackets(t *testing.T) {
	c, server := newTestClient(t)

	count := make(chan int, 8)
	c.RegisterHandler(0x007F, func(data []byte) error {
		count <- len(data)
		return nil
	})

	batch := append(append(notifyTimePacket(), notifyTimePacket()...), notifyTimePacket()...)
	if _, err := server.Write(batch); err != nil {
		t.Fatalf("server write: %v", err)
	}

	got := 0
	deadline := time.Now().Add(2 * time.Second)
	for got < 3 && time.Now().Before(deadline) {
		if err := c.Process(); err != nil {
			t.Fatalf("Process: %v", err)
		}
		for drained := true; drained; {
			select {
			case <-count:
				got++
			default:
				drained = false
			}
		}
		time.Sleep(5 * time.Millisecond)
	}

	if got != 3 {
		t.Errorf("handler fired %d times for 3 batched packets", got)
	}
}

// TestReconnectStartsAFreshReader guards the reader's lifecycle. The client
// walks login to char to map, disconnecting between each, so a stale reader
// left running would deliver the previous connection's bytes into the new one.
func TestReconnectStartsAFreshReader(t *testing.T) {
	c, server := newTestClient(t)

	fired := make(chan int, 4)
	c.RegisterHandler(0x007F, func(data []byte) error {
		fired <- len(data)
		return nil
	})

	if _, err := server.Write(notifyTimePacket()); err != nil {
		t.Fatalf("server write: %v", err)
	}
	if _, ok := waitForHandler(t, c, fired); !ok {
		t.Fatal("first connection never delivered")
	}

	c.Disconnect()

	// Processing a disconnected client must be harmless, not a panic or a read
	// of a closed socket.
	if err := c.Process(); err != nil {
		t.Errorf("Process after Disconnect: %v", err)
	}

	// A second connection must work as well as the first.
	c2, server2 := newTestClient(t)
	c2.RegisterHandler(0x007F, func(data []byte) error {
		fired <- len(data)
		return nil
	})
	if _, err := server2.Write(notifyTimePacket()); err != nil {
		t.Fatalf("second server write: %v", err)
	}
	if _, ok := waitForHandler(t, c2, fired); !ok {
		t.Error("second connection never delivered")
	}
}

// TestProcessReportsServerDisconnect: the loop needs to find out, not hang.
func TestProcessReportsServerDisconnect(t *testing.T) {
	c, server := newTestClient(t)
	_ = server.Close()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := c.Process(); err != nil {
			return // reported, as it should be
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Error("Process never reported the server closing the connection")
}

// TestHandlerRegisteredLateStillSeesPackets is the regression guard for an
// empty-looking map. The server describes every unit around you the moment you
// enter, while the client is still loading it — so those packets arrive
// seconds before the in-game state and its handlers exist. Dropping them
// leaves the map bare until something happens to be re-sent, which for a
// standing NPC is never.
func TestHandlerRegisteredLateStillSeesPackets(t *testing.T) {
	c, server := newTestClient(t)

	// Three arrive with nothing listening.
	for i := 0; i < 3; i++ {
		if _, err := server.Write(notifyTimePacket()); err != nil {
			t.Fatalf("server write: %v", err)
		}
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := c.Process(); err != nil {
			t.Fatalf("Process: %v", err)
		}
		if c.heldCount == 3 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if c.heldCount != 3 {
		t.Fatalf("held %d packets, want 3", c.heldCount)
	}

	// Registering now must deliver the backlog, oldest first.
	fired := make(chan int, 8)
	c.RegisterHandler(0x007F, func(data []byte) error {
		fired <- len(data)
		return nil
	})

	for i := 0; i < 3; i++ {
		select {
		case n := <-fired:
			if n != 6 {
				t.Errorf("held packet %d was %d bytes, want 6", i, n)
			}
		default:
			t.Fatalf("only %d of 3 held packets were delivered", i)
		}
	}
	if c.heldCount != 0 {
		t.Errorf("heldCount = %d after delivery, want 0", c.heldCount)
	}
}

// TestHeldPacketsAreBounded: some ids are never handled at all, and an
// unbounded backlog would grow for the whole session and crowd out the ones
// that will be.
func TestHeldPacketsAreBounded(t *testing.T) {
	c, server := newTestClient(t)

	for i := 0; i < heldPerPacket+20; i++ {
		if _, err := server.Write(notifyTimePacket()); err != nil {
			t.Fatalf("server write: %v", err)
		}
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if err := c.Process(); err != nil {
			t.Fatalf("Process: %v", err)
		}
		if c.heldCount >= heldPerPacket {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if c.heldCount > heldPerPacket {
		t.Errorf("held %d packets of one id, want at most %d", c.heldCount, heldPerPacket)
	}
}

// TestDisconnectDropsHeldPackets: the client walks login to char to map,
// disconnecting between each, and a backlog from one server must never be
// delivered to the state that follows it.
func TestDisconnectDropsHeldPackets(t *testing.T) {
	c, server := newTestClient(t)

	if _, err := server.Write(notifyTimePacket()); err != nil {
		t.Fatalf("server write: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && c.heldCount == 0 {
		if err := c.Process(); err != nil {
			t.Fatalf("Process: %v", err)
		}
		time.Sleep(5 * time.Millisecond)
	}
	if c.heldCount == 0 {
		t.Fatal("nothing was held to begin with")
	}

	c.Disconnect()
	if c.heldCount != 0 {
		t.Errorf("heldCount = %d after Disconnect, want 0", c.heldCount)
	}

	fired := make(chan int, 1)
	c.RegisterHandler(0x007F, func(data []byte) error {
		fired <- len(data)
		return nil
	})
	select {
	case <-fired:
		t.Error("a packet from the previous connection was delivered after it closed")
	default:
	}
}
