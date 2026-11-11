package printer

import (
	"testing"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/google/uuid"
)

type recordingClient struct {
	request PrintRequest
	started bool
}

func (c *recordingClient) Connect() error                              { return nil }
func (c *recordingClient) Disconnect() error                           { return nil }
func (c *recordingClient) GetStatus() (*model.PrinterState, error)     { return nil, nil }
func (c *recordingClient) PauseJob() error                             { return nil }
func (c *recordingClient) ResumeJob() error                            { return nil }
func (c *recordingClient) CancelJob() error                            { return nil }
func (c *recordingClient) SetStatusCallback(func(*model.PrinterState)) {}
func (c *recordingClient) StartJob(request PrintRequest) error {
	c.request = request
	c.started = true
	return nil
}

func TestManagerStartJobNormalizesRemoteDirectory(t *testing.T) {
	t.Parallel()

	printerID := uuid.New()
	client := &recordingClient{}
	manager := NewManager()
	manager.clients[printerID] = client

	err := manager.StartJob(printerID, PrintRequest{Filename: "model.gcode", LocalPath: "/tmp/model.gcode", RemoteDirectory: "/sda1/"})
	if err != nil {
		t.Fatalf("StartJob: %v", err)
	}
	if !client.started {
		t.Fatal("client was not called")
	}
	if client.request.RemoteDirectory != "sda1" {
		t.Fatalf("RemoteDirectory = %q, want sda1", client.request.RemoteDirectory)
	}
}

func TestManagerStartJobRejectsUnsafeRemoteDirectory(t *testing.T) {
	t.Parallel()

	printerID := uuid.New()
	client := &recordingClient{}
	manager := NewManager()
	manager.clients[printerID] = client

	err := manager.StartJob(printerID, PrintRequest{Filename: "model.gcode", LocalPath: "/tmp/model.gcode", RemoteDirectory: "%2e%2e/config"})
	if err == nil {
		t.Fatal("StartJob expected validation error")
	}
	if client.started {
		t.Fatal("unsafe request reached client")
	}
}
