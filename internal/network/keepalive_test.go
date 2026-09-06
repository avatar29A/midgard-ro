package network

import (
	"testing"
	"time"
)

// TestKeepAliveSendsOffTheGameLoop is the point of the whole thing: the packet
// goes out on its own goroutine, so a game loop that never calls Process — a
// display asleep, a screen recording holding the buffer swap — cannot let the
// session time out. Nothing here drives it but the ticker.
func TestKeepAliveSendsOffTheGameLoop(t *testing.T) {
	client, server := newTestClient(t)
	defer client.Disconnect()

	sent := 0
	client.ArmKeepAlive(20*time.Millisecond, func() []byte {
		sent++

		return []byte{0xAA, 0xBB}
	})

	// The server end should see the bytes arrive without anyone calling
	// Process or Update.
	buf := make([]byte, 2)
	_ = server.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := server.Read(buf); err != nil {
		t.Fatalf("no keep-alive arrived off the loop: %v", err)
	}

	if buf[0] != 0xAA || buf[1] != 0xBB {
		t.Errorf("the server read % X, want the keep-alive AA BB", buf)
	}

	if client.LastKeepAliveAt().IsZero() {
		t.Error("the send time was not recorded for the round trip")
	}
}

// TestKeepAliveStopsOnDisconnect: a keep-alive outliving its connection would
// send into a closed socket every tick.
func TestKeepAliveStopsOnDisconnect(t *testing.T) {
	client, server := newTestClient(t)

	fired := make(chan struct{}, 8)
	client.ArmKeepAlive(15*time.Millisecond, func() []byte {
		select {
		case fired <- struct{}{}:
		default:
		}

		return []byte{0x01, 0x02}
	})

	// Let it tick at least once.
	<-fired
	client.Disconnect()
	_ = server.Close()

	// Drain anything already queued, then confirm it goes quiet.
	drain := time.After(60 * time.Millisecond)
	for {
		select {
		case <-fired:
			// A straggler from before Disconnect; keep draining.
		case <-drain:
			select {
			case <-fired:
				t.Error("the keep-alive kept firing after Disconnect")
			default:
			}

			return
		}
	}
}

// TestArmKeepAliveReplacesThePrevious: arming twice must not leave two
// goroutines feeding the socket.
func TestArmKeepAliveReplacesThePrevious(t *testing.T) {
	client, _ := newTestClient(t)
	defer client.Disconnect()

	first := make(chan struct{}, 16)
	client.ArmKeepAlive(10*time.Millisecond, func() []byte {
		select {
		case first <- struct{}{}:
		default:
		}

		return nil
	})

	<-first // it is running

	client.ArmKeepAlive(10*time.Millisecond, func() []byte { return nil })

	// Give the old one time to stop, then confirm it has.
	time.Sleep(40 * time.Millisecond)
	for len(first) > 0 {
		<-first
	}
	time.Sleep(40 * time.Millisecond)

	if len(first) > 0 {
		t.Error("the replaced keep-alive is still firing")
	}
}
