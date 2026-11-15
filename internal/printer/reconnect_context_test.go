package printer

import (
	"bufio"
	"context"
	"crypto/tls"
	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/google/uuid"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"nhooyr.io/websocket"
	"testing"
	"time"
)

func TestHTTPConnectCancellationReachesIO(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	for _, c := range []interface {
		ConnectContext(context.Context) error
		Disconnect() error
	}{NewMoonrakerClient(uuid.New(), server.URL), NewOctoPrintClient(uuid.New(), server.URL, "")} {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
		start := time.Now()
		err := c.ConnectContext(ctx)
		cancel()
		_ = c.Disconnect()
		if err == nil || time.Since(start) > time.Second {
			t.Fatalf("cancellation did not bound I/O: %v", err)
		}
	}
}
func TestBambuTLSHandshakeCancellation(t *testing.T) {
	// The production protocol uses 8883; bind a local ephemeral port and override
	// the dial target via a test-only host? Instead directly test the same custom
	// transport through an injectable broker below.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err == nil {
			defer conn.Close()
			buf := make([]byte, 4096)
			for {
				if _, err = conn.Read(buf); err != nil {
					return
				}
			}
		}
	}()
	c := NewBambuClient(uuid.New(), "127.0.0.1", "fake", "fake")
	c.brokerURL = "ssl://" + listener.Addr().String()
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err := c.ConnectContext(ctx); err == nil {
		t.Fatal("stalled TLS succeeded")
	}
	_ = c.Disconnect()
	if time.Since(start) > time.Second {
		t.Fatal("TLS cancellation exceeded budget")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("TLS connection leaked")
	}
}
func TestManagerRejectsStaleCallbacksAndSnapshotsState(t *testing.T) {
	m := NewManager()
	server := moonrakerServer(t, nil)
	p := &model.Printer{ID: uuid.New(), ConnectionType: model.ConnectionTypeMoonraker, ConnectionURI: server.URL}
	if err := m.Connect(p); err != nil {
		t.Fatal(err)
	}
	m.mu.RLock()
	old := m.clients[p.ID]
	m.mu.RUnlock()
	c := old.(*MoonrakerClient)
	callback := c.statusCallback
	macro := c.macroAutomationCallback
	calls := 0
	m.OnMacroAutomation(func(uuid.UUID) { calls++ })
	original := &model.PrinterState{PrinterID: p.ID, Status: model.PrinterStatusPrinting}
	callback(original)
	original.Status = model.PrinterStatusOffline
	state, _ := m.GetState(p.ID)
	if state.Status != model.PrinterStatusPrinting {
		t.Fatal("manager retained mutable client state")
	}
	m.Disconnect(p.ID)
	callback(&model.PrinterState{PrinterID: p.ID, Status: model.PrinterStatusIdle})
	macro(p.ID)
	if calls != 0 {
		t.Fatal("stale macro accepted")
	}
	if _, err := m.GetState(p.ID); err == nil {
		t.Fatal("stale status resurrected printer")
	}
	m.DisconnectAll()
	callback(original)
	macro(p.ID)
	if calls != 0 {
		t.Fatal("shutdown accepted macro")
	}
}

func TestBambuMQTTHandshakeAndSubscribeCancellation(t *testing.T) {
	for _, phase := range []string{"connack", "suback"} {
		t.Run(phase, func(t *testing.T) {
			certServer := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
			cert := certServer.TLS.Certificates[0]
			certServer.Close()
			listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}})
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			done := make(chan struct{})
			go func() {
				defer close(done)
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				defer conn.Close()
				reader := bufio.NewReader(conn)
				for {
					header, err := reader.ReadByte()
					if err != nil {
						return
					}
					length := 0
					mult := 1
					for {
						b, err := reader.ReadByte()
						if err != nil {
							return
						}
						length += int(b&127) * mult
						if b&128 == 0 {
							break
						}
						mult *= 128
					}
					if _, err := io.CopyN(io.Discard, reader, int64(length)); err != nil {
						return
					}
					if header>>4 == 1 && phase == "suback" {
						if _, err := conn.Write([]byte{0x20, 2, 0, 0}); err != nil {
							return
						}
					}
				}
			}()
			c := NewBambuClient(uuid.New(), "127.0.0.1", "fake", "fake")
			c.brokerURL = "ssl://" + listener.Addr().String()
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			start := time.Now()
			if err := c.ConnectContext(ctx); err == nil {
				t.Fatal("stall succeeded")
			}
			_ = c.Disconnect()
			if time.Since(start) > time.Second {
				t.Fatal("MQTT did not cancel")
			}
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("MQTT connection leaked")
			}
		})
	}
}

func TestMoonrakerWebSocketLossStaysOfflineWithHTTPAlive(t *testing.T) {
	sockets := make(chan *websocket.Conn, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/websocket" {
			conn, err := websocket.Accept(w, r, nil)
			if err == nil {
				sockets <- conn
				_, _, _ = conn.Read(r.Context())
				_ = conn.CloseNow()
			}
			return
		}
		_, _ = w.Write([]byte(`{"result":{"status":{"print_stats":{"state":"standby"}}}}`))
	}))
	defer server.Close()
	c := NewMoonrakerClient(uuid.New(), server.URL)
	defer c.Disconnect()
	updates := make(chan model.PrinterStatus, 10)
	c.SetStatusCallback(func(state *model.PrinterState) { updates <- state.Status })
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := c.ConnectContext(ctx); err != nil {
		t.Fatal(err)
	}
	if status := <-updates; status != model.PrinterStatusIdle {
		t.Fatal(status)
	}
	conn := <-sockets
	_ = conn.CloseNow()
	select {
	case status := <-updates:
		if status != model.PrinterStatusOffline {
			t.Fatal(status)
		}
	case <-time.After(time.Second):
		t.Fatal("websocket loss not reported")
	}
	state, err := c.GetStatus()
	if err != nil || state.Status != model.PrinterStatusOffline {
		t.Fatalf("HTTP hid socket loss: %+v %v", state, err)
	}
}
