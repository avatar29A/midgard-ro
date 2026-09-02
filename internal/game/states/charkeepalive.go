package states

import (
	"time"

	"go.uber.org/zap"

	"github.com/Faultbox/midgard-ro/internal/logger"
	"github.com/Faultbox/midgard-ro/internal/network"
	"github.com/Faultbox/midgard-ro/internal/network/packets"
	"github.com/Faultbox/midgard-ro/internal/trace"
)

// charPingInterval is how often to say something on the character-server
// connection.
//
// The server drops a session that has sent nothing for stall_time seconds,
// 60 by default. Twenty leaves room for two missed ticks before that matters,
// and costs six bytes a time.
const charPingInterval = 20 * time.Second

// charKeepAlive keeps a character-server connection from being dropped while
// a person reads the screen.
//
// Both character select and character creation sit idle far longer than the
// server's patience — picking a name and a hair style is a minute's work on
// its own — and the connection closing underneath looks like the screen
// breaking rather than like a timeout.
//
// The zero value is usable and sends its first ping one interval in, which is
// right: a screen has just sent something to arrive at all.
type charKeepAlive struct {
	last time.Time
}

// tick sends a keep-alive if one is due.
func (k *charKeepAlive) tick(client *network.Client) {
	if client == nil {
		return
	}

	now := time.Now()
	if k.last.IsZero() {
		k.last = now

		return
	}

	if now.Sub(k.last) < charPingInterval {
		return
	}

	k.last = now

	accountID, _, _, _ := client.Session()
	if err := client.Send(packets.EncodePing(accountID)); err != nil {
		logger.Warn("could not send the character-server keep-alive", zap.Error(err))

		return
	}

	trace.Emit(trace.Char, "keepalive", zap.Uint32("accountID", accountID))
}
