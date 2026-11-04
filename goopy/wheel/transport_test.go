package wheel

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// silentProxy forwards TCP until stop is set. After that it keeps the connections
// open and sends nothing, as a dead network path does.
type silentProxy struct {
	ln     net.Listener
	target string
	silent atomic.Bool
}

func newSilentProxy(t *testing.T, target string) *silentProxy {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	p := &silentProxy{ln: ln, target: target}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			up, err := net.Dial("tcp", target)
			if err != nil {
				_ = c.Close()
				continue
			}
			t.Cleanup(func() { _ = c.Close(); _ = up.Close() })
			go p.copy(c, up)
			go p.copy(up, c)
		}
	}()
	return p
}

func (p *silentProxy) copy(dst, src net.Conn) {
	buf := make([]byte, 32<<10)
	for {
		n, err := src.Read(buf)
		if err != nil {
			return
		}
		if !p.silent.Load() {
			if _, err := dst.Write(buf[:n]); err != nil {
				return
			}
		}
	}
}

// readAfterSilence starts a download, makes the network silent after the first
// bytes, and returns how the rest of the read ended.
func readAfterSilence(t *testing.T, transport *http.Transport, wait time.Duration) (finished bool, took time.Duration) {
	t.Helper()
	release := make(chan struct{})
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("first bytes"))
		w.(http.Flusher).Flush()
		<-release
	}))
	srv.EnableHTTP2 = true
	srv.StartTLS()
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(release) })

	proxy := newSilentProxy(t, srv.Listener.Addr().String())
	transport.TLSClientConfig = srv.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
	t.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{Transport: transport}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+proxy.ln.Addr().String()+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	if resp.ProtoMajor != 2 {
		t.Fatalf("protocol = %s, want HTTP/2", resp.Proto)
	}
	if _, err := io.ReadFull(resp.Body, make([]byte, len("first bytes"))); err != nil {
		t.Fatal(err)
	}
	proxy.silent.Store(true)

	start := time.Now()
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		close(done)
	}()
	select {
	case <-done:
		return true, time.Since(start)
	case <-time.After(wait):
		return false, wait
	}
}

func TestTransport_DeadConnectionFailsFast(t *testing.T) {
	finished, took := readAfterSilence(t, newTransport(200*time.Millisecond, 200*time.Millisecond), 10*time.Second)
	if !finished {
		t.Fatalf("download still waiting after %v on a dead connection", took)
	}
}

// TestTransport_WithoutPingsADeadConnectionHangs is the control: it shows the failure
// the pings exist for, so the test above cannot pass for another reason.
func TestTransport_WithoutPingsADeadConnectionHangs(t *testing.T) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if finished, took := readAfterSilence(t, transport, 3*time.Second); finished {
		t.Fatalf("download ended after %v, want it still waiting", took)
	}
}

func TestTransport_IsShared(t *testing.T) {
	first, second := Transport(), Transport()
	if first != second {
		t.Error("Transport returned two transports")
	}
	if got := NewSource().HTTPClient.Transport; got != http.RoundTripper(Transport()) {
		t.Errorf("NewSource transport = %v", got)
	}
}
