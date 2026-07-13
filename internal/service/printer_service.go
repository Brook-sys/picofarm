package service

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Brook-sys/picofarm/internal/bambu"
	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/printer"
	"github.com/Brook-sys/picofarm/internal/realtime"
	"github.com/Brook-sys/picofarm/internal/repository"
	"github.com/Brook-sys/picofarm/internal/validation"
	"github.com/google/uuid"
)

type PrinterService struct {
	repo              *repository.PrinterRepository
	settingsRepo      *repository.SettingsRepository
	printJobRepo      *repository.PrintJobRepository
	queueRepo         *repository.QueueItemRepository
	saleRepo          *repository.SaleRepository
	macroRepo         *repository.PrinterMacroRepository
	manager           *printer.Manager
	hub               *realtime.Hub
	discovery         *printer.Discovery
	bambuCloudRepo    *repository.BambuCloudRepository
	bambuCloud        *bambu.CloudClient
	reconnectMu       sync.Mutex
	reconnecting      map[uuid.UUID]time.Time
	autoReconnectOnce sync.Once
}

// Create creates a new printer.
func (s *PrinterService) Create(ctx context.Context, p *model.Printer) error {
	if p.Name == "" {
		return fmt.Errorf("printer name is required")
	}
	if err := normalizePrinterPrintFolder(p); err != nil {
		return err
	}
	s.ensureFluiddURL(p)
	if err := s.repo.Create(ctx, p); err != nil {
		return err
	}
	if s.settingsRepo != nil {
		printers, _ := s.repo.List(ctx)
		if len(printers) == 1 {
			_ = s.settingsRepo.Set(ctx, "default_printer_id", p.ID.String())
		}
	}

	// Connect printer if not manual and not in maintenance mode
	if p.ConnectionType != model.ConnectionTypeManual && !p.MaintenanceMode {
		go s.manager.Connect(p)
	}

	return nil
}

func (s *PrinterService) GetDefault(ctx context.Context) (*model.Printer, error) {
	printers, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	if len(printers) == 0 {
		return nil, nil
	}
	if len(printers) == 1 {
		return &printers[0], nil
	}
	if s.settingsRepo != nil {
		setting, err := s.settingsRepo.Get(ctx, "default_printer_id")
		if err == nil && setting != nil && setting.Value != "" {
			id, err := uuid.Parse(setting.Value)
			if err == nil {
				p, err := s.repo.GetByID(ctx, id)
				if err == nil && p != nil {
					return p, nil
				}
			}
		}
	}
	return &printers[0], nil
}

func (s *PrinterService) SetDefault(ctx context.Context, id uuid.UUID) error {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if p == nil {
		return fmt.Errorf("printer not found")
	}
	if s.settingsRepo == nil {
		return fmt.Errorf("settings unavailable")
	}
	return s.settingsRepo.Set(ctx, "default_printer_id", id.String())
}

// GetByID retrieves a printer by ID.
func (s *PrinterService) GetByID(ctx context.Context, id uuid.UUID) (*model.Printer, error) {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil || p == nil {
		return p, err
	}
	s.ensureFluiddURL(p)
	return p, nil
}

// List retrieves all printers.
func (s *PrinterService) List(ctx context.Context) ([]model.Printer, error) {
	printers, err := s.repo.List(ctx)
	if err != nil {
		return printers, err
	}
	for i := range printers {
		s.ensureFluiddURL(&printers[i])
	}
	return printers, nil
}

func (s *PrinterService) ensureFluiddURL(p *model.Printer) {
	if p.FluiddURL != "" {
		return
	}
	if p.ConnectionType != model.ConnectionTypeMoonraker || p.ConnectionURI == "" {
		return
	}
	if u, err := url.Parse(p.ConnectionURI); err == nil {
		host := u.Hostname()
		scheme := u.Scheme
		if scheme == "" {
			scheme = "http"
		}
		if host != "" {
			p.FluiddURL = fmt.Sprintf("%s://%s", scheme, host)
		}
	}
}

func normalizePrinterPrintFolder(p *model.Printer) error {
	folder, err := printer.NormalizeRemotePrintFolder(p.DefaultPrintFolder)
	if err != nil {
		return &validation.ValidationError{Errors: []validation.FieldError{{
			Field:   "default_print_folder",
			Message: err.Error(),
		}}}
	}
	p.DefaultPrintFolder = folder
	return nil
}

// Update updates a printer.
func (s *PrinterService) Update(ctx context.Context, p *model.Printer) error {
	if err := normalizePrinterPrintFolder(p); err != nil {
		return err
	}
	s.ensureFluiddURL(p)
	previous, err := s.repo.GetByID(ctx, p.ID)
	if err != nil {
		return err
	}
	if err := s.repo.Update(ctx, p); err != nil {
		return err
	}
	if p.MaintenanceMode {
		s.manager.Disconnect(p.ID)
		return nil
	}
	if previous != nil && previous.MaintenanceMode && p.ConnectionType != model.ConnectionTypeManual {
		go s.manager.Connect(p)
	}
	return nil
}

func (s *PrinterService) Reconnect(ctx context.Context, id uuid.UUID) error {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if p == nil {
		return fmt.Errorf("printer not found")
	}
	if p.ConnectionType == model.ConnectionTypeManual {
		return fmt.Errorf("manual printers do not support reconnection")
	}
	if p.MaintenanceMode {
		return fmt.Errorf("printer is in maintenance mode")
	}
	s.manager.Disconnect(id)
	return s.manager.Connect(p)
}

func (s *PrinterService) ReconnectAsync(ctx context.Context, id uuid.UUID) error {
	s.reconnectMu.Lock()
	if s.reconnecting == nil {
		s.reconnecting = make(map[uuid.UUID]time.Time)
	}
	if started, ok := s.reconnecting[id]; ok && time.Since(started) < 15*time.Second {
		s.reconnectMu.Unlock()
		return nil
	}
	s.reconnecting[id] = time.Now()
	s.reconnectMu.Unlock()

	go func() {
		defer func() {
			s.reconnectMu.Lock()
			delete(s.reconnecting, id)
			s.reconnectMu.Unlock()
		}()
		if err := s.Reconnect(context.Background(), id); err != nil {
			slog.Warn("printer reconnect failed", "printer_id", id, "error", err)
		}
	}()
	return nil
}

func (s *PrinterService) ReconnectOfflinePrinters(ctx context.Context) {
	printers, err := s.repo.List(ctx)
	if err != nil {
		slog.Warn("failed to list printers for auto reconnect", "error", err)
		return
	}
	for i := range printers {
		p := &printers[i]
		if p.ConnectionType == model.ConnectionTypeManual || p.MaintenanceMode {
			continue
		}
		state, err := s.manager.GetState(p.ID)
		if err == nil && state.Status != model.PrinterStatusOffline && state.Status != model.PrinterStatusError {
			continue
		}
		_ = s.ReconnectAsync(ctx, p.ID)
	}
}

func (s *PrinterService) RunMacro(ctx context.Context, id uuid.UUID, macro string) error {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if p == nil {
		return fmt.Errorf("printer not found")
	}
	macro = strings.TrimSpace(macro)
	if macro == "" {
		return fmt.Errorf("macro is required")
	}
	return s.manager.RunMacro(id, macro)
}

func (s *PrinterService) ListMacros(ctx context.Context) ([]model.PrinterMacro, error) {
	return s.macroRepo.List(ctx)
}

func (s *PrinterService) CreateMacro(ctx context.Context, title, command string) (*model.PrinterMacro, error) {
	macro := &model.PrinterMacro{Title: strings.TrimSpace(title), Command: strings.TrimSpace(command)}
	if macro.Title == "" {
		return nil, fmt.Errorf("title is required")
	}
	if macro.Command == "" {
		return nil, fmt.Errorf("command is required")
	}
	if err := s.macroRepo.Create(ctx, macro); err != nil {
		return nil, err
	}
	return macro, nil
}

func (s *PrinterService) UpdateMacro(ctx context.Context, macro *model.PrinterMacro) (*model.PrinterMacro, error) {
	macro.Title = strings.TrimSpace(macro.Title)
	macro.Command = strings.TrimSpace(macro.Command)
	if macro.Title == "" {
		return nil, fmt.Errorf("title is required")
	}
	if macro.Command == "" {
		return nil, fmt.Errorf("command is required")
	}
	if err := s.macroRepo.Update(ctx, macro); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("macro not found")
		}
		return nil, err
	}
	return macro, nil
}

func (s *PrinterService) DeleteMacro(ctx context.Context, id int64) error {
	if err := s.macroRepo.Delete(ctx, id); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("macro not found")
		}
		return err
	}
	return nil
}

// SetMaintenanceMode enables or disables maintenance mode for a printer.
func (s *PrinterService) SetMaintenanceMode(ctx context.Context, id uuid.UUID, maintenanceMode bool) (*model.Printer, error) {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, fmt.Errorf("printer not found")
	}
	p.MaintenanceMode = maintenanceMode
	if err := s.Update(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// Delete removes a printer.
func (s *PrinterService) Delete(ctx context.Context, id uuid.UUID) error {
	s.manager.Disconnect(id)
	return s.repo.Delete(ctx, id)
}

// GetState retrieves real-time state for a printer.
func (s *PrinterService) GetState(ctx context.Context, id uuid.UUID) (*model.PrinterState, error) {
	return s.manager.GetState(id)
}

// GetAllStates retrieves real-time state for all printers.
func (s *PrinterService) GetAllStates(ctx context.Context) map[uuid.UUID]*model.PrinterState {
	return s.manager.GetAllStates()
}

func (s *PrinterService) EmergencyStop(ctx context.Context) []error {
	return s.manager.EmergencyStop()
}

func (s *PrinterService) SetPrintSpeed(ctx context.Context, id uuid.UUID, level int) error {
	if level < 1 || level > 4 {
		return fmt.Errorf("speed level must be between 1 and 4")
	}
	return s.manager.SetPrintSpeed(id, level)
}

func (s *PrinterService) SetFeedRate(ctx context.Context, id uuid.UUID, percent int) error {
	if percent < 25 || percent > 200 {
		return fmt.Errorf("feed rate must be between 25 and 200")
	}
	return s.manager.SetFeedRate(id, percent)
}

func (s *PrinterService) GetCapabilities(ctx context.Context, id uuid.UUID) (model.PrinterCapabilities, error) {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return model.PrinterCapabilities{}, err
	}
	if p == nil {
		return model.PrinterCapabilities{}, fmt.Errorf("printer not found")
	}
	if p.ConnectionType == model.ConnectionTypeManual || p.MaintenanceMode {
		return model.PrinterCapabilities{}, nil
	}
	return s.manager.Capabilities(id)
}

func (s *PrinterService) SetFanSpeed(ctx context.Context, id uuid.UUID, fan string, speed int) error {
	if speed < 0 || speed > 100 {
		return fmt.Errorf("fan speed must be between 0 and 100")
	}
	return s.manager.SetFanSpeed(id, fan, speed)
}

func (s *PrinterService) SetLEDMode(ctx context.Context, id uuid.UUID, mode string) error {
	if mode != "on" && mode != "off" && mode != "flashing" {
		return fmt.Errorf("unsupported LED mode")
	}
	return s.manager.SetLEDMode(id, mode)
}

func (s *PrinterService) SkipObject(ctx context.Context, id uuid.UUID, objectID string) error {
	if objectID == "" {
		return fmt.Errorf("object id is required")
	}
	return s.manager.SkipObject(id, objectID)
}

func (s *PrinterService) Jog(ctx context.Context, id uuid.UUID, axis string, distance float64) error {
	if distance < -100 || distance > 100 {
		return fmt.Errorf("jog distance must be between -100 and 100")
	}
	return s.manager.Jog(id, axis, distance)
}

func (s *PrinterService) SetTemperature(ctx context.Context, id uuid.UUID, heater string, temp float64) error {
	if temp < 0 || temp > 350 {
		return fmt.Errorf("temperature out of range")
	}
	return s.manager.SetTemperature(id, heater, temp)
}

func (s *PrinterService) PlateCleared(ctx context.Context, id uuid.UUID) error {
	return s.manager.PlateCleared(id)
}

func (s *PrinterService) AMSLoad(ctx context.Context, id uuid.UUID, amsID string, slotID string) error {
	if amsID == "" || slotID == "" {
		return fmt.Errorf("ams_id and slot_id are required")
	}
	return s.manager.AMSLoad(id, amsID, slotID)
}

func (s *PrinterService) AMSUnload(ctx context.Context, id uuid.UUID) error {
	return s.manager.AMSUnload(id)
}

func (s *PrinterService) AMSRefresh(ctx context.Context, id uuid.UUID) error {
	return s.manager.AMSRefresh(id)
}

func (s *PrinterService) SetAMSFilamentBackup(ctx context.Context, id uuid.UUID, enabled bool) error {
	return s.manager.SetAMSFilamentBackup(id, enabled)
}

// ListJobs retrieves all print jobs for a printer.
func (s *PrinterService) ListJobs(ctx context.Context, printerID uuid.UUID) ([]model.PrintJob, error) {
	return s.printJobRepo.List(ctx, &printerID, nil)
}

// GetJobStats retrieves job statistics for a printer.
func (s *PrinterService) GetJobStats(ctx context.Context, printerID uuid.UUID) (*repository.JobStats, error) {
	return s.printJobRepo.GetPrinterJobStats(ctx, printerID)
}

// GetPrinterAnalytics computes comprehensive analytics for a printer.
func (s *PrinterService) GetPrinterAnalytics(ctx context.Context, printerID uuid.UUID) (*model.PrinterAnalytics, error) {
	// Fetch printer metadata
	printer, err := s.repo.GetByID(ctx, printerID)
	if err != nil {
		return nil, fmt.Errorf("get printer: %w", err)
	}
	if printer == nil {
		return nil, fmt.Errorf("printer not found")
	}

	// Compute revenue attribution (lifetime)
	revenueCents, err := s.repo.GetPrinterRevenueAttribution(ctx, printerID)
	if err != nil {
		slog.Warn("failed to get revenue attribution", "printer_id", printerID, "error", err)
		revenueCents = 0
	}

	// Compute utilization for 7d, 30d, 90d periods
	now := time.Now()
	periods := []struct {
		label string
		since time.Time
	}{
		{"7d", now.AddDate(0, 0, -7)},
		{"30d", now.AddDate(0, 0, -30)},
		{"90d", now.AddDate(0, 0, -90)},
	}

	var utilizations []model.PrinterUtilization
	for _, p := range periods {
		data, err := s.repo.GetPrinterUtilizationData(ctx, printerID, p.since)
		if err != nil {
			slog.Warn("failed to get utilization data", "period", p.label, "error", err)
			continue
		}

		totalHours := now.Sub(p.since).Hours()
		printingHours := float64(data.CompletedSeconds) / 3600.0
		failedHours := float64(data.FailedSeconds) / 3600.0
		idleHours := totalHours - printingHours - failedHours
		if idleHours < 0 {
			idleHours = 0
		}

		var utilizationPercent float64
		if totalHours > 0 {
			utilizationPercent = (printingHours / totalHours) * 100
		}

		var actualRevenuePerHour int
		if printingHours > 0 {
			// Proportionally attribute revenue to this period based on total printing hours
			healthData, _ := s.repo.GetPrinterHealthData(ctx, printerID)
			if healthData != nil && healthData.TotalSeconds > 0 {
				totalPrintingHours := float64(healthData.TotalSeconds) / 3600.0
				periodRevenueShare := (printingHours / totalPrintingHours) * float64(revenueCents)
				actualRevenuePerHour = int(periodRevenueShare / printingHours)
			}
		}

		utilizations = append(utilizations, model.PrinterUtilization{
			Period:                     p.label,
			TotalHours:                 totalHours,
			PrintingHours:              printingHours,
			FailedHours:                failedHours,
			IdleHours:                  idleHours,
			UtilizationPercent:         utilizationPercent,
			ConfiguredCostPerHourCents: printer.CostPerHourCents,
			ActualRevenuePerHourCents:  actualRevenuePerHour,
		})
	}

	// Compute health metrics
	healthData, err := s.repo.GetPrinterHealthData(ctx, printerID)
	if err != nil {
		return nil, fmt.Errorf("get health data: %w", err)
	}

	failureBreakdown, err := s.repo.GetPrinterFailureBreakdown(ctx, printerID)
	if err != nil {
		slog.Warn("failed to get failure breakdown", "printer_id", printerID, "error", err)
		failureBreakdown = make(map[string]int)
	}

	var failureRate float64
	if healthData.TotalJobs > 0 {
		failureRate = float64(healthData.FailedJobs) / float64(healthData.TotalJobs) * 100
	}

	var avgJobDuration int
	var avgCost int
	if healthData.CompletedJobs > 0 {
		avgJobDuration = healthData.TotalSeconds / healthData.CompletedJobs
		avgCost = healthData.TotalCostCents / healthData.CompletedJobs
	}

	health := &model.PrinterHealth{
		TotalJobs:          healthData.TotalJobs,
		CompletedJobs:      healthData.CompletedJobs,
		FailedJobs:         healthData.FailedJobs,
		FailureRate:        failureRate,
		AvgJobDurationSec:  avgJobDuration,
		AvgCostCents:       avgCost,
		TotalMaterialGrams: healthData.TotalMaterialGrams,
		TotalCostCents:     healthData.TotalCostCents,
		TotalRevenueCents:  revenueCents,
		FailureBreakdown:   failureBreakdown,
	}

	// Compute ROI metrics
	totalPrintingHours := float64(healthData.TotalSeconds) / 3600.0
	printerAgeHours := now.Sub(printer.CreatedAt).Hours()

	var revenuePerHour, costPerHour, netPerHour int
	if totalPrintingHours > 0 {
		revenuePerHour = int(float64(revenueCents) / totalPrintingHours)
		costPerHour = int(float64(healthData.TotalCostCents) / totalPrintingHours)
		netPerHour = revenuePerHour - costPerHour
	}

	lifetimeProfit := revenueCents - healthData.TotalCostCents - printer.PurchasePriceCents

	var hoursToBreakEven float64
	breakEvenReached := lifetimeProfit >= 0
	if !breakEvenReached && netPerHour > 0 {
		remainingToBreakEven := -lifetimeProfit
		hoursToBreakEven = float64(remainingToBreakEven) / float64(netPerHour)
	} else if breakEvenReached {
		// Calculate when break-even was reached
		if netPerHour > 0 {
			hoursToBreakEven = float64(printer.PurchasePriceCents) / float64(netPerHour)
		}
	}

	roi := &model.PrinterROI{
		PurchasePriceCents:  printer.PurchasePriceCents,
		TotalRevenueCents:   revenueCents,
		TotalCostCents:      healthData.TotalCostCents,
		LifetimeProfitCents: lifetimeProfit,
		TotalPrintingHours:  totalPrintingHours,
		RevenuePerHourCents: revenuePerHour,
		CostPerHourCents:    costPerHour,
		NetPerHourCents:     netPerHour,
		HoursToBreakEven:    hoursToBreakEven,
		PrinterAgeHours:     printerAgeHours,
		BreakEvenReached:    breakEvenReached,
	}

	return &model.PrinterAnalytics{
		Utilization: utilizations,
		ROI:         roi,
		Health:      health,
	}, nil
}

// DiscoverPrinters scans the network for printers.
func (s *PrinterService) DiscoverPrinters(ctx context.Context) ([]printer.DiscoveredPrinter, error) {
	discovered, err := s.discovery.QuickScan(ctx)
	if err != nil {
		return nil, err
	}

	// Mark printers that are already added
	existing, _ := s.repo.List(ctx)
	existingURIs := make(map[string]bool)
	for _, p := range existing {
		existingURIs[p.ConnectionURI] = true
	}

	for i := range discovered {
		host := discovered[i].Host
		uri := fmt.Sprintf("http://%s:%d", host, discovered[i].Port)
		// Check both full URI and bare host (Bambu printers store just the IP)
		if existingURIs[uri] || existingURIs[host] {
			discovered[i].AlreadyAdded = true
		}
	}

	return discovered, nil
}

// ConnectAllPrinters loads all printers from the database and connects
// non-manual printers. Called at startup to restore connections.
// For bambu_cloud printers, credentials are refreshed from the stored
// cloud auth so that token updates from later logins are picked up.
func (s *PrinterService) StartAutoReconnect(ctx context.Context) {
	s.autoReconnectOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					s.ReconnectOfflinePrinters(ctx)
				}
			}
		}()
	})
}

func (s *PrinterService) ConnectAllPrinters(ctx context.Context) {
	printers, err := s.repo.List(ctx)
	if err != nil {
		slog.Error("failed to load printers for reconnection", "error", err)
		return
	}

	// Load and validate cloud auth once if any cloud printers exist
	var cloudAuth *model.BambuCloudAuth
	for _, p := range printers {
		if p.ConnectionType == model.ConnectionTypeBambuCloud {
			cloudAuth, _ = s.bambuCloudRepo.Get(ctx)
			if cloudAuth != nil {
				// Check if token needs refresh
				if cloudAuth.IsExpired(tokenRefreshBuffer) && cloudAuth.CanRefresh() {
					slog.Info("refreshing expired bambu cloud token at startup")
					refreshedAuth, err := s.refreshBambuToken(ctx, cloudAuth)
					if err != nil {
						slog.Error("failed to refresh bambu cloud token at startup", "error", err)
						cloudAuth = nil // Mark as unavailable
					} else {
						cloudAuth = refreshedAuth
						slog.Info("bambu cloud token refreshed successfully at startup")
					}
				} else if cloudAuth.IsExpired(0) {
					slog.Warn("bambu cloud token expired and no refresh token available")
					cloudAuth = nil
				}
			}
			break
		}
	}

	for i := range printers {
		p := &printers[i]
		if p.ConnectionType == model.ConnectionTypeManual {
			continue
		}

		// For cloud printers, inject the latest auth credentials
		if p.ConnectionType == model.ConnectionTypeBambuCloud {
			if cloudAuth == nil {
				slog.Warn("skipping cloud printer — no valid Bambu Cloud credentials",
					"printer_id", p.ID, "name", p.Name)
				continue
			}
			p.ConnectionURI = cloudAuth.MQTTUsername
			p.APIKey = cloudAuth.AccessToken
		}

		slog.Info("reconnecting printer", "id", p.ID, "name", p.Name, "type", p.ConnectionType)
		go s.manager.Connect(p)
	}
}

// refreshBambuToken refreshes the Bambu Cloud access token.
func (s *PrinterService) refreshBambuToken(ctx context.Context, auth *model.BambuCloudAuth) (*model.BambuCloudAuth, error) {
	if s.bambuCloud == nil {
		return nil, fmt.Errorf("bambu cloud client not configured")
	}

	resp, err := s.bambuCloud.RefreshToken(auth.RefreshToken)
	if err != nil {
		return nil, err
	}

	// Calculate new expiration time
	var expiresAt *time.Time
	if resp.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second)
		expiresAt = &t
	}

	// Update auth with new tokens
	auth.AccessToken = resp.AccessToken
	if resp.RefreshToken != "" {
		auth.RefreshToken = resp.RefreshToken
	}
	auth.ExpiresAt = expiresAt

	// Persist updated credentials
	if err := s.bambuCloudRepo.Upsert(ctx, auth); err != nil {
		return nil, fmt.Errorf("failed to save refreshed credentials: %w", err)
	}

	return auth, nil
}

// BambuCloudService handles Bambu Cloud authentication and device management.
type BambuCloudService struct {
	cloud       *bambu.CloudClient
	repo        *repository.BambuCloudRepository
	printerRepo *repository.PrinterRepository
	printerMgr  *printer.Manager
}

// NewBambuCloudService creates a new BambuCloudService.
func NewBambuCloudService(repo *repository.BambuCloudRepository, printerRepo *repository.PrinterRepository, printerMgr *printer.Manager, cloud *bambu.CloudClient) *BambuCloudService {
	return &BambuCloudService{
		cloud:       cloud,
		repo:        repo,
		printerRepo: printerRepo,
		printerMgr:  printerMgr,
	}
}

// Login authenticates with Bambu Cloud. Returns true if a verification code is needed.
func (s *BambuCloudService) Login(ctx context.Context, email, password string) (needsCode bool, err error) {
	resp, err := s.cloud.Login(email, password)
	if err != nil {
		return false, err
	}

	if resp.LoginType == "verifyCode" || resp.LoginType == "tfa" {
		// Need email verification code
		if err := s.cloud.RequestVerifyCode(email); err != nil {
			return false, fmt.Errorf("failed to request verification code: %w", err)
		}
		return true, nil
	}

	// Direct login succeeded — store credentials
	return false, s.storeAuth(ctx, email, resp)
}

// VerifyCode completes login with a verification code.
func (s *BambuCloudService) VerifyCode(ctx context.Context, email, code string) error {
	resp, err := s.cloud.LoginWithCode(email, code)
	if err != nil {
		return err
	}
	return s.storeAuth(ctx, email, resp)
}

// storeAuth fetches the MQTT username and persists credentials.
func (s *BambuCloudService) storeAuth(ctx context.Context, email string, resp *bambu.LoginResponse) error {
	mqttUsername, err := s.cloud.GetUsername(resp.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to get MQTT username: %w", err)
	}

	var expiresAt *time.Time
	if resp.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second)
		expiresAt = &t
	}

	auth := &model.BambuCloudAuth{
		Email:        email,
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		MQTTUsername: mqttUsername,
		ExpiresAt:    expiresAt,
	}
	return s.repo.Upsert(ctx, auth)
}

// tokenRefreshBuffer is how long before expiration we should refresh the token.
const tokenRefreshBuffer = 5 * time.Minute

// GetValidAuth retrieves auth credentials, automatically refreshing if expired.
// Returns an error if not authenticated or if refresh fails.
func (s *BambuCloudService) GetValidAuth(ctx context.Context) (*model.BambuCloudAuth, error) {
	auth, err := s.repo.Get(ctx)
	if err != nil {
		return nil, err
	}
	if auth == nil {
		return nil, fmt.Errorf("not authenticated with Bambu Cloud")
	}

	// Check if token is expired or about to expire
	if auth.IsExpired(tokenRefreshBuffer) {
		slog.Info("bambu cloud token expired or expiring soon, attempting refresh",
			"expires_at", auth.ExpiresAt,
			"can_refresh", auth.CanRefresh())

		if !auth.CanRefresh() {
			return nil, fmt.Errorf("bambu cloud token expired and no refresh token available - please re-login")
		}

		// Attempt refresh
		refreshedAuth, err := s.refreshToken(ctx, auth)
		if err != nil {
			slog.Error("failed to refresh bambu cloud token", "error", err)
			return nil, fmt.Errorf("failed to refresh token: %w - please re-login", err)
		}
		auth = refreshedAuth
		slog.Info("bambu cloud token refreshed successfully", "new_expires_at", auth.ExpiresAt)
	}

	return auth, nil
}

// refreshToken uses the refresh token to get new credentials.
func (s *BambuCloudService) refreshToken(ctx context.Context, auth *model.BambuCloudAuth) (*model.BambuCloudAuth, error) {
	resp, err := s.cloud.RefreshToken(auth.RefreshToken)
	if err != nil {
		return nil, err
	}

	// Calculate new expiration time
	var expiresAt *time.Time
	if resp.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second)
		expiresAt = &t
	}

	// Update auth with new tokens (keep existing email and mqtt username)
	auth.AccessToken = resp.AccessToken
	if resp.RefreshToken != "" {
		auth.RefreshToken = resp.RefreshToken
	}
	auth.ExpiresAt = expiresAt

	// Persist updated credentials
	if err := s.repo.Upsert(ctx, auth); err != nil {
		return nil, fmt.Errorf("failed to save refreshed credentials: %w", err)
	}

	return auth, nil
}

// GetDevices fetches the device list from Bambu Cloud.
func (s *BambuCloudService) GetDevices(ctx context.Context) ([]bambu.CloudDevice, error) {
	auth, err := s.GetValidAuth(ctx)
	if err != nil {
		return nil, err
	}
	return s.cloud.GetDevices(auth.AccessToken)
}

// GetStoredAuth returns the stored authentication info (without exposing the token).
func (s *BambuCloudService) GetStoredAuth(ctx context.Context) (*model.BambuCloudAuth, error) {
	return s.repo.Get(ctx)
}

// AddDevice creates a printer from a cloud device and connects it.
func (s *BambuCloudService) AddDevice(ctx context.Context, devID string) (*model.Printer, error) {
	auth, err := s.GetValidAuth(ctx)
	if err != nil {
		return nil, err
	}

	devices, err := s.cloud.GetDevices(auth.AccessToken)
	if err != nil {
		return nil, err
	}

	// Find the requested device
	var device *bambu.CloudDevice
	for i := range devices {
		if devices[i].DevID == devID {
			device = &devices[i]
			break
		}
	}
	if device == nil {
		return nil, fmt.Errorf("device %s not found in cloud account", devID)
	}

	// Create the printer record
	p := &model.Printer{
		Name:           device.Name,
		Model:          device.DevProductName,
		Manufacturer:   "Bambu Lab",
		ConnectionType: model.ConnectionTypeBambuCloud,
		ConnectionURI:  auth.MQTTUsername, // Store MQTT username here
		APIKey:         auth.AccessToken,  // Store auth token here
		SerialNumber:   device.DevID,
		NozzleDiameter: device.NozzleDiameter,
	}
	if p.NozzleDiameter == 0 {
		p.NozzleDiameter = 0.4
	}

	if err := s.printerRepo.Create(ctx, p); err != nil {
		return nil, err
	}

	// Connect via cloud MQTT
	go s.printerMgr.Connect(p)

	return p, nil
}

// Logout clears stored Bambu Cloud credentials.
func (s *BambuCloudService) Logout(ctx context.Context) error {
	return s.repo.Delete(ctx)
}

// MaterialService handles material business logic.
