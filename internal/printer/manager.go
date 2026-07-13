package printer

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/google/uuid"
)

// Client defines the interface for printer communication.
type Client interface {
	// Connect establishes connection to the printer.
	Connect() error
	// Disconnect closes the connection.
	Disconnect() error
	// GetStatus retrieves current printer status.
	GetStatus() (*model.PrinterState, error)
	// StartJob sends a file to print.
	StartJob(request PrintRequest) error
	// PauseJob pauses the current print.
	PauseJob() error
	// ResumeJob resumes a paused print.
	ResumeJob() error
	// CancelJob cancels the current print.
	CancelJob() error
	// SetStatusCallback sets callback for status updates.
	SetStatusCallback(func(*model.PrinterState))
}

// StatusChangeCallback is called when a printer's status changes.
// Receives the new state and the previous state (nil if first update).
type StatusChangeCallback func(newState *model.PrinterState, oldState *model.PrinterState)

type MacroRunner interface {
	RunMacro(name string) error
}

type MacroAutomationListener interface {
	SetMacroAutomationCallback(func(printerID uuid.UUID))
}

type CapabilityProvider interface {
	Capabilities() model.PrinterCapabilities
}

type FeedRateController interface {
	SetFeedRate(percent int) error
}

type AdvancedController interface {
	SetPrintSpeed(level int) error
	SetFanSpeed(fan string, speed int) error
	SetLEDMode(mode string) error
	SkipObject(objectID string) error
	Jog(axis string, distance float64) error
	SetTemperature(heater string, temp float64) error
	PlateCleared() error
	AMSLoad(amsID string, slotID string) error
	AMSUnload() error
	AMSRefresh() error
	SetAMSFilamentBackup(enabled bool) error
}

// Manager manages connections to multiple printers.
type Manager struct {
	mu                      sync.RWMutex
	clients                 map[uuid.UUID]Client
	states                  map[uuid.UUID]*model.PrinterState
	broadcaster             model.Broadcaster
	statusCallbacks         []StatusChangeCallback
	macroAutomationCallback func(uuid.UUID)
}

// NewManager creates a new printer manager.
func NewManager() *Manager {
	return &Manager{
		clients:         make(map[uuid.UUID]Client),
		states:          make(map[uuid.UUID]*model.PrinterState),
		statusCallbacks: []StatusChangeCallback{},
	}
}

// OnStatusChange registers a callback for printer status changes.
// This allows services to react to printer state transitions (e.g., detect failures).
func (m *Manager) OnStatusChange(cb StatusChangeCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.statusCallbacks = append(m.statusCallbacks, cb)
}

// SetBroadcaster sets the broadcaster for real-time updates.
func (m *Manager) SetBroadcaster(b model.Broadcaster) {
	m.broadcaster = b
}

// OnMacroAutomation registers a callback for printer macro automation events (e.g. queue empty / next job).
func (m *Manager) OnMacroAutomation(cb func(printerID uuid.UUID)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.macroAutomationCallback = cb
}

// broadcast sends an event to all connected WebSocket clients.
func (m *Manager) broadcast(eventType string, data interface{}) {
	if m.broadcaster != nil {
		m.broadcaster.Broadcast(model.BroadcastEvent{
			Type: eventType,
			Data: data,
		})
	}
}

func attachCapabilities(state *model.PrinterState, client Client) *model.PrinterState {
	if state == nil {
		return nil
	}
	if cp, ok := client.(CapabilityProvider); ok {
		state.Capabilities = cp.Capabilities()
		return state
	}
	state.Capabilities = model.PrinterCapabilities{
		CanStartPrint: true,
		CanPause:      true,
		CanResume:     true,
		CanCancel:     true,
		CanUploadFile: true,
	}
	return state
}

// Connect establishes connection to a printer.
// Skips connection if printer is in maintenance mode.
func (m *Manager) Connect(p *model.Printer) error {
	if p.MaintenanceMode {
		m.mu.Lock()
		m.states[p.ID] = &model.PrinterState{
			PrinterID: p.ID,
			Status:    model.PrinterStatusOffline,
			UpdatedAt: time.Now(),
		}
		m.mu.Unlock()
		slog.Info("skipping connect for printer in maintenance mode", "printer_id", p.ID)
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Create appropriate client based on connection type
	var client Client
	switch p.ConnectionType {
	case model.ConnectionTypeOctoPrint:
		client = NewOctoPrintClient(p.ID, p.ConnectionURI, p.APIKey)
	case model.ConnectionTypeBambuLAN:
		client = NewBambuClient(p.ID, p.ConnectionURI, p.APIKey, p.SerialNumber)
	case model.ConnectionTypeBambuCloud:
		// For cloud printers: ConnectionURI = MQTT username, APIKey = auth token
		client = NewBambuCloudClient(p.ID, p.SerialNumber, p.ConnectionURI, p.APIKey)
	case model.ConnectionTypeMoonraker:
		client = NewMoonrakerClient(p.ID, p.ConnectionURI)
	case model.ConnectionTypeManual:
		// No client for manual printers
		m.states[p.ID] = &model.PrinterState{
			PrinterID: p.ID,
			Status:    model.PrinterStatusOffline,
			UpdatedAt: time.Now(),
		}
		return nil
	default:
		return fmt.Errorf("unsupported connection type: %s", p.ConnectionType)
	}

	// Set up status callback
	client.SetStatusCallback(func(state *model.PrinterState) {
		state = attachCapabilities(state, client)
		m.mu.Lock()
		oldState := m.states[p.ID]
		m.states[p.ID] = state
		callbacks := make([]StatusChangeCallback, len(m.statusCallbacks))
		copy(callbacks, m.statusCallbacks)
		m.mu.Unlock()

		slog.Info("printer status update", "printer_id", p.ID, "status", state.Status, "progress", state.Progress)

		// Broadcast state change to WebSocket clients
		m.broadcast(model.EventPrinterStateUpdated, state)

		// Notify registered status change listeners
		for _, cb := range callbacks {
			cb(state, oldState)
		}
	})

	// Set up macro automation callback if supported
	if mal, ok := client.(MacroAutomationListener); ok {
		mal.SetMacroAutomationCallback(func(printerID uuid.UUID) {
			m.mu.RLock()
			cb := m.macroAutomationCallback
			m.mu.RUnlock()
			if cb != nil {
				cb(printerID)
			}
		})
	}

	// Connect
	if err := client.Connect(); err != nil {
		slog.Error("failed to connect to printer", "printer_id", p.ID, "error", err)
		m.states[p.ID] = &model.PrinterState{
			PrinterID: p.ID,
			Status:    model.PrinterStatusOffline,
			UpdatedAt: time.Now(),
		}
		return err
	}

	m.clients[p.ID] = client

	// Get initial status
	if state, err := client.GetStatus(); err == nil {
		state = attachCapabilities(state, client)
		m.states[p.ID] = state
		// Broadcast initial state
		m.broadcast(model.EventPrinterStateUpdated, state)
	}

	slog.Info("connected to printer", "printer_id", p.ID, "type", p.ConnectionType)

	// Broadcast printer connected event
	m.broadcast(model.EventPrinterConnected, map[string]interface{}{
		"printer_id": p.ID,
	})

	return nil
}

// Disconnect closes connection to a printer.
func (m *Manager) Disconnect(id uuid.UUID) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if client, ok := m.clients[id]; ok {
		client.Disconnect() //nolint:errcheck // best-effort cleanup
		delete(m.clients, id)
	}
	delete(m.states, id)

	// Broadcast printer disconnected event
	m.broadcast(model.EventPrinterDisconnected, map[string]interface{}{
		"printer_id": id,
	})
}

// DisconnectAll closes all printer connections. Used during graceful shutdown.
func (m *Manager) DisconnectAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, client := range m.clients {
		slog.Info("disconnecting printer", "printer_id", id)
		client.Disconnect() //nolint:errcheck // best-effort shutdown cleanup
		delete(m.clients, id)
	}
	// Clear all states
	for id := range m.states {
		delete(m.states, id)
	}
	slog.Info("all printers disconnected")
}

// GetState retrieves current state for a printer.
func (m *Manager) GetState(id uuid.UUID) (*model.PrinterState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if state, ok := m.states[id]; ok {
		return state, nil
	}
	return nil, fmt.Errorf("printer not found")
}

// GetAllStates retrieves current state for all printers.
func (m *Manager) GetAllStates() map[uuid.UUID]*model.PrinterState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[uuid.UUID]*model.PrinterState)
	for id, state := range m.states {
		result[id] = state
	}
	return result
}

// StartJob sends a print job to a printer.
func (m *Manager) StartJob(printerID uuid.UUID, request PrintRequest) error {
	remoteDirectory, err := NormalizeRemotePrintFolder(request.RemoteDirectory)
	if err != nil {
		return fmt.Errorf("invalid remote print folder: %w", err)
	}
	request.RemoteDirectory = remoteDirectory

	m.mu.RLock()
	client, ok := m.clients[printerID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("printer not connected")
	}

	return client.StartJob(request)
}

// PauseJob pauses the current print on a printer.
func (m *Manager) PauseJob(printerID uuid.UUID) error {
	m.mu.RLock()
	client, ok := m.clients[printerID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("printer not connected")
	}

	return client.PauseJob()
}

// ResumeJob resumes a paused print on a printer.
func (m *Manager) ResumeJob(printerID uuid.UUID) error {
	m.mu.RLock()
	client, ok := m.clients[printerID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("printer not connected")
	}

	return client.ResumeJob()
}

// CancelJob cancels the current print on a printer.
func (m *Manager) CancelJob(printerID uuid.UUID) error {
	m.mu.RLock()
	client, ok := m.clients[printerID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("printer not connected")
	}

	return client.CancelJob()
}

// EmergencyStop cancels all active prints on every connected printer.
func (m *Manager) EmergencyStop() []error {
	m.mu.RLock()
	clients := make(map[uuid.UUID]Client, len(m.clients))
	for id, c := range m.clients {
		clients[id] = c
	}
	m.mu.RUnlock()

	var errs []error
	for id, c := range clients {
		if mr, ok := c.(MacroRunner); ok {
			if err := mr.RunMacro("M112"); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", id, err))
			}
			continue
		}
		if err := c.CancelJob(); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", id, err))
		}
	}
	return errs
}

func (m *Manager) RunMacro(printerID uuid.UUID, name string) error {
	m.mu.RLock()
	c, ok := m.clients[printerID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("printer not connected")
	}
	if mr, ok := c.(MacroRunner); ok {
		return mr.RunMacro(name)
	}
	return fmt.Errorf("macros not supported for this printer")
}

func (m *Manager) Capabilities(printerID uuid.UUID) (model.PrinterCapabilities, error) {
	m.mu.RLock()
	c, ok := m.clients[printerID]
	m.mu.RUnlock()
	if !ok {
		return model.PrinterCapabilities{}, fmt.Errorf("printer not connected")
	}
	if cp, ok := c.(CapabilityProvider); ok {
		return cp.Capabilities(), nil
	}
	return model.PrinterCapabilities{
		CanStartPrint: true,
		CanPause:      true,
		CanResume:     true,
		CanCancel:     true,
		CanUploadFile: true,
	}, nil
}

func (m *Manager) SetFeedRate(printerID uuid.UUID, percent int) error {
	m.mu.RLock()
	c, ok := m.clients[printerID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("printer not connected")
	}
	if frc, ok := c.(FeedRateController); ok {
		return frc.SetFeedRate(percent)
	}
	return fmt.Errorf("feed rate control not supported for this printer")
}

// Advanced control methods (best-effort, return error if unsupported)
func (m *Manager) SetPrintSpeed(printerID uuid.UUID, level int) error {
	m.mu.RLock()
	c, ok := m.clients[printerID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("printer not connected")
	}
	if adv, ok := c.(AdvancedController); ok {
		return adv.SetPrintSpeed(level)
	}
	return fmt.Errorf("advanced controls not supported for this printer")
}

func (m *Manager) SetFanSpeed(printerID uuid.UUID, fan string, speed int) error {
	m.mu.RLock()
	c, ok := m.clients[printerID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("printer not connected")
	}
	if adv, ok := c.(AdvancedController); ok {
		return adv.SetFanSpeed(fan, speed)
	}
	return fmt.Errorf("advanced controls not supported for this printer")
}

func (m *Manager) SetLEDMode(printerID uuid.UUID, mode string) error {
	m.mu.RLock()
	c, ok := m.clients[printerID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("printer not connected")
	}
	if adv, ok := c.(AdvancedController); ok {
		return adv.SetLEDMode(mode)
	}
	return fmt.Errorf("advanced controls not supported for this printer")
}

func (m *Manager) SkipObject(printerID uuid.UUID, objectID string) error {
	m.mu.RLock()
	c, ok := m.clients[printerID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("printer not connected")
	}
	if adv, ok := c.(AdvancedController); ok {
		return adv.SkipObject(objectID)
	}
	return fmt.Errorf("advanced controls not supported for this printer")
}

func (m *Manager) Jog(printerID uuid.UUID, axis string, distance float64) error {
	m.mu.RLock()
	c, ok := m.clients[printerID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("printer not connected")
	}
	if adv, ok := c.(AdvancedController); ok {
		return adv.Jog(axis, distance)
	}
	return fmt.Errorf("advanced controls not supported for this printer")
}

func (m *Manager) SetTemperature(printerID uuid.UUID, heater string, temp float64) error {
	m.mu.RLock()
	c, ok := m.clients[printerID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("printer not connected")
	}
	if adv, ok := c.(AdvancedController); ok {
		return adv.SetTemperature(heater, temp)
	}
	return fmt.Errorf("advanced controls not supported for this printer")
}

func (m *Manager) PlateCleared(printerID uuid.UUID) error {
	m.mu.RLock()
	c, ok := m.clients[printerID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("printer not connected")
	}
	if adv, ok := c.(AdvancedController); ok {
		return adv.PlateCleared()
	}
	return fmt.Errorf("advanced controls not supported for this printer")
}

func (m *Manager) AMSLoad(printerID uuid.UUID, amsID string, slotID string) error {
	m.mu.RLock()
	c, ok := m.clients[printerID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("printer not connected")
	}
	if adv, ok := c.(AdvancedController); ok {
		return adv.AMSLoad(amsID, slotID)
	}
	return fmt.Errorf("AMS controls not supported for this printer")
}

func (m *Manager) AMSUnload(printerID uuid.UUID) error {
	m.mu.RLock()
	c, ok := m.clients[printerID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("printer not connected")
	}
	if adv, ok := c.(AdvancedController); ok {
		return adv.AMSUnload()
	}
	return fmt.Errorf("AMS controls not supported for this printer")
}

func (m *Manager) AMSRefresh(printerID uuid.UUID) error {
	m.mu.RLock()
	c, ok := m.clients[printerID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("printer not connected")
	}
	if adv, ok := c.(AdvancedController); ok {
		return adv.AMSRefresh()
	}
	return fmt.Errorf("AMS controls not supported for this printer")
}

func (m *Manager) SetAMSFilamentBackup(printerID uuid.UUID, enabled bool) error {
	m.mu.RLock()
	c, ok := m.clients[printerID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("printer not connected")
	}
	if adv, ok := c.(AdvancedController); ok {
		return adv.SetAMSFilamentBackup(enabled)
	}
	return fmt.Errorf("AMS controls not supported for this printer")
}
