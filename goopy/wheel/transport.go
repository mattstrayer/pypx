package wheel

import (
	"net/http"
	"sync"
	"time"
)

const (
	// http2PingAfter is how long a connection may stay silent before it is pinged.
	http2PingAfter = 10 * time.Second
	// http2PingWait is how long a ping may go unanswered before the connection closes.
	http2PingWait = 5 * time.Second
)

// Transport returns the transport for downloads from PyPI, shared by all clients.
// All requests to one host share one HTTP/2 connection, so if that connection dies
// without closing, every download on it waits for its own timeout. Without pings,
// nothing notices sooner. Here a dead connection fails within about 15 seconds.
var Transport = sync.OnceValue(func() *http.Transport {
	return newTransport(http2PingAfter, http2PingWait)
})

func newTransport(pingAfter, pingWait time.Duration) *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.HTTP2 = &http.HTTP2Config{SendPingTimeout: pingAfter, PingTimeout: pingWait}
	return t
}
