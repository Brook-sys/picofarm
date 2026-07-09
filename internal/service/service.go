package service

import (
	"time"

	"github.com/Brook-sys/picofarm/internal/bambu"
	"github.com/Brook-sys/picofarm/internal/printer"
	"github.com/Brook-sys/picofarm/internal/realtime"
	"github.com/Brook-sys/picofarm/internal/repository"
	"github.com/Brook-sys/picofarm/internal/saleschannel"
	"github.com/Brook-sys/picofarm/internal/storage"
	"github.com/google/uuid"
)

// Services holds all service instances.
type Services struct {
	Projects         *ProjectService
	Parts            *PartService
	Designs          *DesignService
	Printers         *PrinterService
	Materials        *MaterialService
	Spools           *SpoolService
	PrintJobs        *PrintJobService
	Files            *FileService
	Expenses         *ExpenseService
	Sales            *SaleService
	Stats            *StatsService
	Etsy             *EtsyService
	Squarespace      *SquarespaceService
	SalesChannels    *saleschannel.Registry
	SalesChannelData *repository.SalesChannelRepository
	BambuCloud       *BambuCloudService
	Settings         *SettingsService
	ProjectSupplies  *ProjectSupplyService
	Backup           *BackupService
	Dispatcher       *DispatcherService
	// New services for feature gaps
	Orders        *OrderService
	Alerts        *AlertService
	Tags          *TagService
	Shopify       *ShopifyService
	Timeline      *TimelineService
	Tasks         *TaskService
	Feedback      *FeedbackService
	Customers     *CustomerService
	Quotes        *QuoteService
	Cameras       *CameraService
	Timelapses    *TimelapseService
	PrintArchives *PrintArchiveService
	Queue         *QueueService
	GCodeLibrary  *GCodeLibraryService
	STLLibrary    *STLLibraryService
	Notifications *NotificationService
	Slicer        *SlicerService
	ModelImport   *ModelImportService
	Thingiverse   *ThingiverseImportService
	PrinterFiles  *PrinterFileService
}

// EtsyConfig holds Etsy OAuth configuration.
type EtsyConfig struct {
	ClientID    string
	RedirectURI string
}

// ServicesConfig holds all service configuration.
type ServicesConfig struct {
	Etsy EtsyConfig
}

// NewServices creates all service instances.
func NewServices(repos *repository.Repositories, store storage.Storage, printerMgr *printer.Manager, hub *realtime.Hub) *Services {
	// Create shared Bambu cloud client
	bambuCloudClient := bambu.NewCloudClient()

	services := &Services{
		Projects:        &ProjectService{repo: repos.Projects, printJobRepo: repos.PrintJobs, printerRepo: repos.Printers, spoolRepo: repos.Spools, designRepo: repos.Designs, tagRepo: repos.Tags, saleRepo: repos.Sales, partRepo: repos.Parts, supplyRepo: repos.ProjectSupplies, repos: repos, printerMgr: printerMgr, hub: hub, storage: store},
		Parts:           &PartService{repo: repos.Parts, designRepo: repos.Designs, projectRepo: repos.Projects, tagRepo: repos.Tags, stlRepo: repos.STLLibrary},
		Designs:         &DesignService{repo: repos.Designs, partRepo: repos.Parts, projectRepo: repos.Projects, tagRepo: repos.Tags, fileRepo: repos.Files, gcodeRepo: repos.GCodeLibrary, stlRepo: repos.STLLibrary, storage: store},
		Printers:        &PrinterService{repo: repos.Printers, settingsRepo: repos.Settings, printJobRepo: repos.PrintJobs, queueRepo: repos.QueueItems, saleRepo: repos.Sales, macroRepo: repos.PrinterMacros, manager: printerMgr, hub: hub, discovery: printer.NewDiscovery(), bambuCloudRepo: repos.BambuCloud, bambuCloud: bambuCloudClient, reconnecting: make(map[uuid.UUID]time.Time)},
		Materials:       &MaterialService{repo: repos.Materials},
		Spools:          &SpoolService{repo: repos.Spools},
		PrintJobs:       &PrintJobService{repo: repos.PrintJobs, printerRepo: repos.Printers, designRepo: repos.Designs, spoolRepo: repos.Spools, materialRepo: repos.Materials, projectRepo: repos.Projects, queueRepo: repos.QueueItems, printerMgr: printerMgr, hub: hub, storage: store},
		Files:           &FileService{repo: repos.Files, storage: store},
		Expenses:        &ExpenseService{repo: repos.Expenses, materialRepo: repos.Materials, spoolRepo: repos.Spools, fileRepo: repos.Files, settingsRepo: repos.Settings, repos: repos, storage: store},
		Sales:           &SaleService{repo: repos.Sales, taskRepo: repos.Tasks},
		Stats:           &StatsService{expenseRepo: repos.Expenses, saleRepo: repos.Sales, printJobRepo: repos.PrintJobs, queueRepo: repos.QueueItems, spoolRepo: repos.Spools},
		Etsy:            nil, // Initialize separately with config
		Squarespace:     nil, // Initialize separately with config
		BambuCloud:      NewBambuCloudService(repos.BambuCloud, repos.Printers, printerMgr, bambuCloudClient),
		Settings:        &SettingsService{repo: repos.Settings},
		ProjectSupplies: &ProjectSupplyService{repo: repos.ProjectSupplies, materialRepo: repos.Materials},
	}
	// Wire cross-service dependencies
	services.Stats.projectService = services.Projects

	// Initialize Squarespace
	services.Squarespace = NewSquarespaceService(repos.Squarespace)
	services.Queue = NewQueueService(repos, store, printerMgr, hub)

	// Initialize Dispatcher service
	services.Dispatcher = NewDispatcherService(
		repos.Dispatch,
		repos.AutoDispatchSettings,
		repos.PrintJobs,
		repos.Printers,
		services.PrintJobs,
		printerMgr,
		hub,
		services.Settings,
		services.Queue,
	)
	services.Dispatcher.Init()

	// Initialize new services for feature gaps
	services.Orders = NewOrderService(repos.Orders, repos.Projects, repos.PrintJobs, hub)
	services.Alerts = NewAlertService(repos.Spools, repos.Materials, repos.Orders, repos.AlertDismissals, hub)
	services.Tags = NewTagService(repos.Tags, repos.Parts, repos.Designs)
	services.Shopify = NewShopifyService(repos.Shopify, services.Orders, hub)
	services.SalesChannels = mustNewSalesChannelRegistry(services)
	services.SalesChannelData = repos.SalesChannels
	services.Timeline = NewTimelineService(repos.Orders, repos.Tasks, repos.Projects, repos.PrintJobs)
	services.Tasks = NewTaskService(repos.Tasks, repos.Projects, repos.PrintJobs, repos.Parts, repos.TaskChecklist, repos.Designs, hub)
	services.Feedback = &FeedbackService{repo: repos.Feedback}
	services.Customers = NewCustomerService(repos.Customers, hub)
	services.Quotes = NewQuoteService(repos.Quotes, repos.Customers, repos.Orders, repos, hub)
	services.Cameras = &CameraService{repo: repos.Cameras, printerRepo: repos.Printers}
	services.Timelapses = &TimelapseService{repo: repos.Timelapses}
	services.PrintArchives = &PrintArchiveService{repo: repos.PrintArchives}
	services.GCodeLibrary = NewGCodeLibraryService(repos, store, services.Queue)
	services.STLLibrary = NewSTLLibraryService(repos, store)
	services.Notifications = NewNotificationService(repos.Notifications)
	services.Queue.SetNotificationService(services.Notifications)
	services.Slicer = NewSlicerService(services.Settings, repos, store, services.GCodeLibrary)
	services.ModelImport = NewModelImportService(services.Projects, services.Parts, services.Designs, services.STLLibrary, services.Files, repos.Tags)
	services.Thingiverse = NewThingiverseImportService(services.Settings, services.STLLibrary)
	services.PrinterFiles = NewPrinterFileService(repos.Printers)
	services.GCodeLibrary.SetPrinterFileService(services.PrinterFiles)

	// Wire job completion callback to auto-complete checklist items
	services.PrintJobs.SetOnJobCompleted(services.Tasks.HandleJobCompleted)

	// Wire task repo to order service (needed for ProcessItem)
	services.Orders.SetTaskRepo(repos.Tasks)

	return services
}

// NewServicesWithEtsy creates all service instances including Etsy integration.
func NewServicesWithEtsy(repos *repository.Repositories, store storage.Storage, printerMgr *printer.Manager, hub *realtime.Hub, etsyConfig EtsyConfig) *Services {
	services := NewServices(repos, store, printerMgr, hub)
	services.Etsy = NewEtsyService(repos.Etsy, etsyConfig.ClientID, etsyConfig.RedirectURI, services.Settings)
	services.SalesChannels = mustNewSalesChannelRegistry(services)
	return services
}

// NewServicesWithConfig creates all service instances with full configuration.
func NewServicesWithConfig(repos *repository.Repositories, store storage.Storage, printerMgr *printer.Manager, hub *realtime.Hub, config ServicesConfig) *Services {
	services := NewServices(repos, store, printerMgr, hub)
	services.Etsy = NewEtsyService(repos.Etsy, config.Etsy.ClientID, config.Etsy.RedirectURI, services.Settings)
	services.SalesChannels = mustNewSalesChannelRegistry(services)
	return services
}

func mustNewSalesChannelRegistry(services *Services) *saleschannel.Registry {
	registry, err := NewSalesChannelRegistry(
		NewEtsySalesChannelProvider(services.Etsy),
		NewSquarespaceSalesChannelProvider(services.Squarespace),
		NewShopifySalesChannelProvider(services.Shopify),
	)
	if err != nil {
		panic(err)
	}
	return registry
}

// SetBackupService sets the backup service (must be called after DB is available).
func (s *Services) SetBackupService(backup *BackupService) {
	s.Backup = backup
}

// ProjectService handles project business logic.
