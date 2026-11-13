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
	// closed is set by DisconnectAll during graceful shutdown. Connect refuses
	// to open new connections afterwards, so a reconnect goroutine that lost the
	// race with shutdown cannot resurrect a printer.
	closed bool

	// printerLocks serializes Connect/Disconnect on the same printer. It is
	// deliberately separate from mu: those calls block on the network for as
	// long as the client's own timeout, and holding mu for that long would stall
	// every status read and every other printer's connection attempt.
	printerLocksMu sync.Mutex
	printerLocks   map[uuid.UUID]*sync.Mutex
}

// SupportsConnectionType reports whether the manager knows how to build a
// client for this connection type. Callers that retry connections in the
// background (auto reconnect) use it to skip printers that could never
// succeed instead of retrying forever.
func SupportsConnectionType(t model.ConnectionType) bool {
	switch t {
	case model.ConnectionTypeOctoPrint,
		model.ConnectionTypeBambuLAN,
		model.ConnectionTypeBambuCloud,
		model.ConnectionTypeMoonraker:
		return true
	default:
		return false
	}
}

// NewManager creates a new printer manager.
func NewManager() *Manager {
	return &Manager{
		clients:         make(map[uuid.UUID]Client),
		states:          make(map[uuid.UUID]*model.PrinterState),
		statusCallbacks: []StatusChangeCallback{},
		printerLocks:    make(map[uuid.UUID]*sync.Mutex),
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

// lockPrinter serializes operations on a single printer and returns the unlock
// function. A new mutex is created on first use and kept forever: the map only
// grows with the number of printers ever connected, and deleting an entry while
// another goroutine holds it would silently allow two operations to overlap.
func (m *Manager) lockPrinter(id uuid.UUID) func() {
	m.printerLocksMu.Lock()
	if m.printerLocks == nil {
		m.printerLocks = make(map[uuid.UUID]*sync.Mutex)
	}
	lock, ok := m.printerLocks[id]
	if !ok {
		lock = &sync.Mutex{}
		m.printerLocks[id] = lock
	}
	m.printerLocksMu.Unlock()

	lock.Lock()
	return lock.Unlock
}

// setOffline records a printer as offline without touching clients.
func (m *Manager) setOffline(id uuid.UUID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.states[id] = &model.PrinterState{
		PrinterID: id,
		Status:    model.PrinterStatusOffline,
		UpdatedAt: time.Now(),
	}
}

// setState stores a state and broadcasts it.
func (m *Manager) setState(id uuid.UUID, state *model.PrinterState) {
	m.mu.Lock()
	m.states[id] = state
	m.mu.Unlock()
	m.broadcast(model.EventPrinterStateUpdated, state)
}

// storeClient installs a client for a printer, closing a previous one instead
// of leaking it. Callers must not hold mu. It reports whether the client was
// installed, which is false when the manager is shutting down.
func (m *Manager) storeClient(id uuid.UUID, client Client) bool {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		// The connection is already open, so dropping the client without
		// closing it would leak it exactly during shutdown.
		slog.Info("manager is shutting down, closing printer connection", "printer_id", id)
		_ = client.Disconnect()
		return false
	}
	replaced := m.clients[id]
	m.clients[id] = client
	m.mu.Unlock()

	if replaced != nil && replaced != client {
		// Two Connect calls for the same printer (a retry racing a manual
		// reconnect) would otherwise leave the first connection open forever.
		if err := replaced.Disconnect(); err != nil {
			slog.Warn("failed to close replaced printer connection", "printer_id", id, "error", err)
		}
	}
	return true
}

// buildClient creates the client for a printer. No network I/O happens here.
func buildClient(p *model.Printer) (Client, error) {
	switch p.ConnectionType {
	case model.ConnectionTypeOctoPrint:
		return NewOctoPrintClient(p.ID, p.ConnectionURI, p.APIKey), nil
	case model.ConnectionTypeBambuLAN:
		return NewBambuClient(p.ID, p.ConnectionURI, p.APIKey, p.SerialNumber), nil
	case model.ConnectionTypeBambuCloud:
		// For cloud printers: ConnectionURI = MQTT username, APIKey = auth token
		return NewBambuCloudClient(p.ID, p.SerialNumber, p.ConnectionURI, p.APIKey), nil
	case model.ConnectionTypeMoonraker:
		return NewMoonrakerClient(p.ID, p.ConnectionURI), nil
	default:
		return nil, fmt.Errorf("unsupported connection type: %s", p.ConnectionType)
	}
}

// Connect establishes connection to a printer.
// Skips connection if printer is in maintenance mode.
//
// mu is never held across network I/O. Connect, GetStatus and Disconnect can
// block for the client's whole timeout, and the previous code held mu for that
// long: with several offline printers, one slow printer froze status reads and
// every other connection attempt for seconds to minutes.
func (m *Manager) Connect(p *model.Printer) error {
	if p == nil {
		return fmt.Errorf("printer is required")
	}
	if p.MaintenanceMode {
		m.setOffline(p.ID)
		slog.Info("skipping connect for printer in maintenance mode", "printer_id", p.ID)
		return nil
	}

	// Only one connection attempt per printer at a time, and never a global
	// serialization point.
	unlock := m.lockPrinter(p.ID)
	defer unlock()

	m.mu.RLock()
	closed := m.closed
	m.mu.RUnlock()
	if closed {
		return fmt.Errorf("printer manager is shutting down")
	}

	if p.ConnectionType == model.ConnectionTypeManual {
		// No client for manual printers.
		m.setOffline(p.ID)
		return nil
	}

	client, err := buildClient(p)
	if err != nil {
		// Unsupported connection types (e.g. chitu) never reach a client, so
		// they must not keep a stale state around either.
		slog.Error("cannot build printer client", "printer_id", p.ID, "type", p.ConnectionType, "error", err)
		m.setOffline(p.ID)
		return err
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

	// Connect (network, no manager lock held)
	if err := client.Connect(); err != nil {
		slog.Error("failed to connect to printer", "printer_id", p.ID, "error", err)
		m.setOffline(p.ID)
		return err
	}

	if !m.storeClient(p.ID, client) {
		return fmt.Errorf("printer manager is shutting down")
	}

	// Get initial status (network, no manager lock held)
	if state, err := client.GetStatus(); err == nil && state != nil {
		state = attachCapabilities(state, client)
		m.setState(p.ID, state)
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
	unlock := m.lockPrinter(id)
	defer unlock()

	m.mu.Lock()
	client, ok := m.clients[id]
	delete(m.clients, id)
	delete(m.states, id)
	m.mu.Unlock()

	if ok {
		// Best effort, and outside mu: a client can block while closing.
		client.Disconnect() //nolint:errcheck // best-effort cleanup
	}

	// Broadcast printer disconnected event
	m.broadcast(model.EventPrinterDisconnected, map[string]interface{}{
		"printer_id": id,
	})
}

// DisconnectAll closes all printer connections. Used during graceful shutdown.
// Connections close in parallel because each one can take as long as the
// client's own close timeout.
func (m *Manager) DisconnectAll() {
	m.mu.Lock()
	clients := m.clients
	m.clients = make(map[uuid.UUID]Client)
	m.states = make(map[uuid.UUID]*model.PrinterState)
	m.closed = true
	m.mu.Unlock()

	if len(clients) == 0 {
		slog.Info("all printers disconnected")
		return
	}

	var wg sync.WaitGroup
	for id, client := range clients {
		wg.Add(1)
		go func(id uuid.UUID, client Client) {
			defer wg.Done()
			slog.Info("disconnecting printer", "printer_id", id)
			client.Disconnect() //nolint:errcheck // best-effort shutdown cleanup
		}(id, client)
	}
	wg.Wait()
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
