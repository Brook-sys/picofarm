package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/Brook-sys/picofarm/internal/etsy"
	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/repository"
	"github.com/google/uuid"
)

// EtsyService handles Etsy OAuth and integration business logic.
type EtsyService struct {
	repo        *repository.EtsyRepository
	client      *etsy.Client
	settingsSvc *SettingsService

	// webhookSecret is the signing secret supplied at startup (environment).
	// When empty the secret is read from settings on each request.
	webhookSecret string
}

// EtsyWebhookSecretKey is the settings key that stores the Etsy webhook signing secret.
const EtsyWebhookSecretKey = "etsy_webhook_secret"

// EtsyRedirectURIKey is the settings key that stores the OAuth redirect URI.
const EtsyRedirectURIKey = "etsy_redirect_uri"

// ErrInvalidRedirectURI is returned when the operator supplies a redirect URI
// that cannot be used in an OAuth request. Rejecting it here gives a 400 in the
// API and a message in the UI, instead of a rejected authorization later, with
// no context about which value was wrong.
var ErrInvalidRedirectURI = errors.New("redirect URI must be an absolute http(s) URL")

// NewEtsyService creates a new EtsyService.
func NewEtsyService(repo *repository.EtsyRepository, clientID, redirectURI string, settingsSvc *SettingsService) *EtsyService {
	var client *etsy.Client
	if clientID != "" {
		client = etsy.NewClient(clientID, redirectURI)
	}
	return &EtsyService{
		repo:        repo,
		client:      client,
		settingsSvc: settingsSvc,
	}
}

// SetWebhookSecret sets the webhook signing secret supplied by startup
// configuration. An empty value keeps the settings-based lookup in place.
func (s *EtsyService) SetWebhookSecret(secret string) {
	if s == nil {
		return
	}
	s.webhookSecret = secret
}

// GetWebhookSecret resolves the Etsy webhook signing secret.
// Order: startup configuration (ETSY_WEBHOOK_SECRET) then the etsy_webhook_secret
// setting. An empty result means no secret is configured, so signature
// verification cannot be enforced.
func (s *EtsyService) GetWebhookSecret(ctx context.Context) (string, error) {
	if s == nil {
		return "", nil
	}
	if s.webhookSecret != "" {
		return s.webhookSecret, nil
	}
	if s.settingsSvc == nil {
		return "", nil
	}
	setting, err := s.settingsSvc.Get(ctx, EtsyWebhookSecretKey)
	if err != nil {
		return "", err
	}
	if setting == nil {
		return "", nil
	}
	return setting.Value, nil
}

// Configure sets the Etsy client ID at runtime and persists it to settings.
// An empty redirect URI falls back to etsy.DefaultRedirectURI (port 8084, the
// documented default port) instead of the port 8080 value older builds used.
// Configure is intentionally callable again on an already configured
// integration: it is the only way for the operator to fix a keystring or a
// redirect URI after the fact.
func (s *EtsyService) Configure(ctx context.Context, clientID, redirectURI string) error {
	if clientID == "" {
		return fmt.Errorf("client_id is required")
	}
	if redirectURI == "" {
		redirectURI = etsy.DefaultRedirectURI
	}
	if err := validateEtsyRedirectURI(redirectURI); err != nil {
		return err
	}
	if redirectURI == etsy.LegacyRedirectURI {
		// Keep it working, but make the operator see that this value is exactly
		// what older builds shipped by mistake.
		slog.Warn("Etsy redirect URI matches the legacy default; only use this if it is registered in the Etsy developer console",
			"redirect_uri", redirectURI)
	}
	s.client = etsy.NewClient(clientID, redirectURI)

	if s.settingsSvc != nil {
		if err := s.settingsSvc.Set(ctx, "etsy_client_id", clientID); err != nil {
			return fmt.Errorf("saving client ID: %w", err)
		}
		if err := s.settingsSvc.Set(ctx, EtsyRedirectURIKey, redirectURI); err != nil {
			return fmt.Errorf("saving redirect URI: %w", err)
		}
	}
	return nil
}

// validateEtsyRedirectURI rejects values that would make the OAuth request
// fail or be impossible to read in logs.
func validateEtsyRedirectURI(redirectURI string) error {
	trimmed := strings.TrimSpace(redirectURI)
	if trimmed != redirectURI {
		return fmt.Errorf("%w: has surrounding whitespace", ErrInvalidRedirectURI)
	}
	parsed, err := url.Parse(redirectURI)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidRedirectURI, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%w: scheme must be http or https, got %q", ErrInvalidRedirectURI, parsed.Scheme)
	}
	if parsed.Host == "" {
		return fmt.Errorf("%w: host is missing", ErrInvalidRedirectURI)
	}
	return nil
}

// RepairLegacyRedirectURI rewrites a redirect URI persisted by an older build
// of PicoFarm. Those builds defaulted to port 8080, so a connection configured
// through the UI permanently failed with "The requested redirect URL is not
// permitted." and the only fix was editing the database by hand.
//
// The value can no longer be corrected from the UI alone when Etsy is already
// "configured", so this repair runs on startup before the client is built.
// Safe to call on every boot and a no-op when nothing is persisted or the
// operator set a value deliberately.
func (s *EtsyService) RepairLegacyRedirectURI(ctx context.Context) error {
	if s == nil || s.settingsSvc == nil {
		return nil
	}
	setting, err := s.settingsSvc.Get(ctx, EtsyRedirectURIKey)
	if err != nil {
		return fmt.Errorf("reading Etsy redirect URI: %w", err)
	}
	if setting == nil || setting.Value == "" {
		// Nothing persisted: the startup default applies.
		return nil
	}
	if setting.Value != etsy.LegacyRedirectURI {
		return nil
	}
	if err := s.settingsSvc.Set(ctx, EtsyRedirectURIKey, etsy.DefaultRedirectURI); err != nil {
		return fmt.Errorf("rewriting legacy Etsy redirect URI: %w", err)
	}
	slog.Info("repaired legacy Etsy redirect URI persisted by an older build",
		"from", etsy.LegacyRedirectURI, "to", etsy.DefaultRedirectURI)
	return nil
}

// RedirectURI returns the OAuth redirect URI currently configured, or an empty
// string when Etsy is not configured. Exposed so the API can tell the operator
// exactly which value has to be registered in the Etsy developer console.
func (s *EtsyService) RedirectURI() string {
	if s == nil || s.client == nil {
		return ""
	}
	return s.client.RedirectURI()
}

// IsConfigured returns true if Etsy OAuth is configured.
func (s *EtsyService) IsConfigured() bool {
	return s.client != nil
}

// StartOAuth initiates the OAuth flow and returns the authorization URL.
func (s *EtsyService) StartOAuth(ctx context.Context) (string, error) {
	if s.client == nil {
		return "", fmt.Errorf("etsy integration not configured")
	}

	// Clean up any expired states
	if err := s.repo.CleanupExpiredStates(ctx); err != nil {
		slog.Warn("failed to cleanup expired OAuth states", "error", err)
	}

	// Generate PKCE values
	state, err := etsy.GenerateState()
	if err != nil {
		return "", fmt.Errorf("generating state: %w", err)
	}

	codeVerifier, err := etsy.GenerateCodeVerifier()
	if err != nil {
		return "", fmt.Errorf("generating code verifier: %w", err)
	}

	codeChallenge := etsy.GenerateCodeChallenge(codeVerifier)

	// Save state for callback verification
	oauthState := &model.EtsyOAuthState{
		State:        state,
		CodeVerifier: codeVerifier,
		// Recorded so the callback can tell whether the operator reconfigured
		// the redirect URI in the middle of the flow. Etsy requires the exact
		// same value in the token exchange, so a change surfaces as an opaque
		// error unless we detect it here.
		RedirectURI: s.client.RedirectURI(),
	}
	if err := s.repo.SaveOAuthState(ctx, oauthState); err != nil {
		return "", fmt.Errorf("saving OAuth state: %w", err)
	}

	// Generate authorization URL
	authURL := s.client.GenerateAuthURL(state, codeChallenge)

	slog.Info("started Etsy OAuth flow", "state", state[:8]+"...")
	return authURL, nil
}

// HandleCallback processes the OAuth callback and saves the integration.
func (s *EtsyService) HandleCallback(ctx context.Context, state, code string) (*model.EtsyIntegration, error) {
	if s.client == nil {
		return nil, fmt.Errorf("etsy integration not configured")
	}

	// Retrieve and validate state
	oauthState, err := s.repo.GetOAuthState(ctx, state)
	if err != nil {
		return nil, fmt.Errorf("retrieving OAuth state: %w", err)
	}
	if oauthState == nil {
		return nil, fmt.Errorf("invalid or expired OAuth state")
	}

	// Check if state is not too old (10 minutes max)
	if time.Since(oauthState.CreatedAt) > 10*time.Minute {
		s.repo.DeleteOAuthState(ctx, state)
		return nil, fmt.Errorf("OAuth state expired")
	}

	// The authorization code can only be exchanged with the same redirect URI
	// that produced it. If the operator reconfigured the app in between, the
	// exchange fails with an opaque Etsy error; fail fast with a message that
	// names both values instead.
	//
	// States created by older builds carry an empty RedirectURI, so the check
	// only applies to states that recorded one.
	if oauthState.RedirectURI != "" && oauthState.RedirectURI != s.client.RedirectURI() {
		s.repo.DeleteOAuthState(ctx, state)
		return nil, fmt.Errorf("redirect URI changed after the authorization started (was %s, now %s): reconfigure the Etsy redirect URI and start the connection again",
			oauthState.RedirectURI, s.client.RedirectURI())
	}

	// Exchange code for tokens. %w keeps the structured Etsy error in the chain
	// so the API layer can forward Etsy's own description to the operator.
	tokenResp, err := s.client.ExchangeCode(ctx, code, oauthState.CodeVerifier)
	if err != nil {
		s.repo.DeleteOAuthState(ctx, state)
		return nil, fmt.Errorf("exchanging code: %w", err)
	}

	// Get shop information
	shop, err := s.client.GetShop(ctx, tokenResp.AccessToken)
	if err != nil {
		s.repo.DeleteOAuthState(ctx, state)
		return nil, fmt.Errorf("getting shop info: %w", err)
	}

	// Save integration
	integration := &model.EtsyIntegration{
		ShopID:         shop.ShopID,
		ShopName:       shop.ShopName,
		UserID:         shop.UserID,
		AccessToken:    tokenResp.AccessToken,
		RefreshToken:   tokenResp.RefreshToken,
		TokenExpiresAt: time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
		Scopes:         etsy.DefaultScopes,
		IsActive:       true,
	}
	if err := s.repo.SaveIntegration(ctx, integration); err != nil {
		s.repo.DeleteOAuthState(ctx, state)
		return nil, fmt.Errorf("saving integration: %w", err)
	}

	// Clean up OAuth state
	s.repo.DeleteOAuthState(ctx, state)

	slog.Info("Etsy shop connected", "shop_id", shop.ShopID, "shop_name", shop.ShopName)
	return integration, nil
}

// GetStatus returns the current Etsy integration status.
func (s *EtsyService) GetStatus(ctx context.Context) (*model.EtsyIntegration, error) {
	return s.repo.GetIntegration(ctx)
}

// Disconnect removes the Etsy integration.
func (s *EtsyService) Disconnect(ctx context.Context) error {
	if err := s.repo.DeleteIntegration(ctx); err != nil {
		return fmt.Errorf("deleting integration: %w", err)
	}
	slog.Info("Etsy shop disconnected")
	return nil
}

// RefreshTokenIfNeeded refreshes the access token if it's about to expire.
// Should be called before making API requests.
func (s *EtsyService) RefreshTokenIfNeeded(ctx context.Context) error {
	if s.client == nil {
		return fmt.Errorf("etsy integration not configured")
	}

	integration, err := s.repo.GetIntegration(ctx)
	if err != nil {
		return fmt.Errorf("getting integration: %w", err)
	}
	if integration == nil {
		return fmt.Errorf("no Etsy integration found")
	}

	// Refresh if token expires in less than 5 minutes
	if time.Until(integration.TokenExpiresAt) > 5*time.Minute {
		return nil // Token is still valid
	}

	slog.Info("refreshing Etsy access token", "shop_id", integration.ShopID)

	tokenResp, err := s.client.RefreshToken(ctx, integration.RefreshToken)
	if err != nil {
		return fmt.Errorf("refreshing token: %w", err)
	}

	expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	if err := s.repo.UpdateTokens(ctx, tokenResp.AccessToken, tokenResp.RefreshToken, expiresAt); err != nil {
		return fmt.Errorf("updating tokens: %w", err)
	}

	slog.Info("Etsy access token refreshed", "shop_id", integration.ShopID)
	return nil
}

// GetAuthenticatedClient returns the Etsy client and a valid access token.
// Refreshes the token if needed.
func (s *EtsyService) GetAuthenticatedClient(ctx context.Context) (*etsy.Client, string, error) {
	if err := s.RefreshTokenIfNeeded(ctx); err != nil {
		return nil, "", err
	}

	integration, err := s.repo.GetIntegration(ctx)
	if err != nil {
		return nil, "", err
	}
	if integration == nil {
		return nil, "", fmt.Errorf("no Etsy integration found")
	}

	return s.client, integration.AccessToken, nil
}

// ---- Receipt/Order Methods ----

// SyncReceipts fetches new receipts from Etsy and stores them locally.
func (s *EtsyService) SyncReceipts(ctx context.Context) (*model.SyncResult, error) {
	client, token, err := s.GetAuthenticatedClient(ctx)
	if err != nil {
		return nil, err
	}

	integration, err := s.repo.GetIntegration(ctx)
	if err != nil {
		return nil, err
	}

	// Get sync state to determine last sync time
	syncState, err := s.repo.GetSyncState(ctx, integration.ShopID)
	if err != nil {
		return nil, fmt.Errorf("getting sync state: %w", err)
	}

	opts := etsy.ReceiptQueryOptions{
		Limit: 100,
	}

	// If we have a last sync time, only fetch receipts since then
	if syncState != nil && syncState.LastReceiptSyncAt != nil {
		opts.MinCreated = syncState.LastReceiptSyncAt.Unix()
	}

	receipts, err := client.GetReceipts(ctx, token, integration.ShopID, opts)
	if err != nil {
		return nil, fmt.Errorf("fetching receipts: %w", err)
	}

	result := &model.SyncResult{
		TotalFetched: len(receipts),
	}

	for _, apiReceipt := range receipts {
		// Check if receipt already exists
		existing, err := s.repo.GetReceiptByEtsyID(ctx, apiReceipt.ReceiptID)
		if err != nil {
			slog.Error("error checking existing receipt", "receipt_id", apiReceipt.ReceiptID, "error", err)
			result.Errors++
			continue
		}

		receipt := convertAPIReceiptToModel(apiReceipt, integration.ShopID)

		if existing != nil {
			receipt.ID = existing.ID
			receipt.IsProcessed = existing.IsProcessed
			receipt.ProjectID = existing.ProjectID
			receipt.CreatedAt = existing.CreatedAt
			result.Updated++
		} else {
			result.Created++
		}

		if err := s.repo.SaveReceipt(ctx, receipt); err != nil {
			slog.Error("error saving receipt", "receipt_id", apiReceipt.ReceiptID, "error", err)
			result.Errors++
			continue
		}

		// Save receipt items (transactions)
		for _, tx := range apiReceipt.Transactions {
			item := convertAPITransactionToItem(tx, receipt.ID)
			if err := s.repo.SaveReceiptItem(ctx, item); err != nil {
				slog.Error("error saving receipt item", "transaction_id", tx.TransactionID, "error", err)
			}
		}
	}

	// Update sync state
	now := time.Now()
	if syncState == nil {
		syncState = &model.EtsySyncState{
			ShopID: integration.ShopID,
		}
	}
	syncState.LastReceiptSyncAt = &now
	if err := s.repo.SaveSyncState(ctx, syncState); err != nil {
		slog.Warn("failed to update sync state", "error", err)
	}

	// Update integration last sync
	s.repo.UpdateLastSync(ctx)

	slog.Info("receipt sync completed", "fetched", result.TotalFetched, "created", result.Created, "updated", result.Updated)
	return result, nil
}

// convertAPIReceiptToModel converts an API receipt to a model receipt.
func convertAPIReceiptToModel(api etsy.APIReceipt, shopID int64) *model.EtsyReceipt {
	receipt := &model.EtsyReceipt{
		EtsyReceiptID:             api.ReceiptID,
		EtsyShopID:                shopID,
		BuyerUserID:               api.BuyerUserID,
		BuyerEmail:                api.BuyerEmail,
		Name:                      api.Name,
		Status:                    api.Status,
		MessageFromBuyer:          api.MessageFromBuyer,
		IsShipped:                 api.IsShipped,
		IsPaid:                    api.IsPaid,
		IsGift:                    api.IsGift,
		GiftMessage:               api.GiftMessage,
		GrandtotalCents:           etsy.MoneyToCents(api.Grandtotal),
		SubtotalCents:             etsy.MoneyToCents(api.Subtotal),
		TotalPriceCents:           etsy.MoneyToCents(api.TotalPrice),
		TotalShippingCostCents:    etsy.MoneyToCents(api.TotalShippingCost),
		TotalTaxCostCents:         etsy.MoneyToCents(api.TotalTaxCost),
		DiscountCents:             etsy.MoneyToCents(api.DiscountAmt),
		Currency:                  api.Grandtotal.CurrencyCode,
		ShippingName:              api.Name,
		ShippingAddressFirstLine:  api.FirstLine,
		ShippingAddressSecondLine: api.SecondLine,
		ShippingCity:              api.City,
		ShippingState:             api.State,
		ShippingZip:               api.Zip,
		ShippingCountryCode:       api.CountryISO,
		SyncedAt:                  time.Now(),
	}

	if api.CreateTimestamp > 0 {
		t := time.Unix(api.CreateTimestamp, 0)
		receipt.CreateTimestamp = &t
	}
	if api.UpdateTimestamp > 0 {
		t := time.Unix(api.UpdateTimestamp, 0)
		receipt.UpdateTimestamp = &t
	}

	return receipt
}

// convertAPITransactionToItem converts an API transaction to a receipt item.
func convertAPITransactionToItem(tx etsy.APITransaction, receiptID uuid.UUID) *model.EtsyReceiptItem {
	item := &model.EtsyReceiptItem{
		EtsyReceiptItemID: tx.TransactionID,
		ReceiptID:         receiptID,
		EtsyListingID:     tx.ListingID,
		EtsyTransactionID: tx.TransactionID,
		Title:             tx.Title,
		Description:       tx.Description,
		Quantity:          tx.Quantity,
		PriceCents:        etsy.MoneyToCents(tx.Price),
		ShippingCostCents: etsy.MoneyToCents(tx.ShippingCost),
		SKU:               tx.SKU,
		IsDigital:         tx.IsDigital,
	}

	// Convert variations to JSON
	if len(tx.Variations) > 0 {
		if data, err := json.Marshal(tx.Variations); err == nil {
			item.Variations = data
		}
	}

	return item
}

// ListReceipts retrieves stored receipts with optional filtering.
func (s *EtsyService) ListReceipts(ctx context.Context, processed *bool, limit, offset int) ([]model.EtsyReceipt, error) {
	receipts, err := s.repo.ListReceipts(ctx, processed, limit, offset)
	if err != nil {
		return nil, err
	}

	// Load items for each receipt
	for i := range receipts {
		items, err := s.repo.GetReceiptItems(ctx, receipts[i].ID)
		if err != nil {
			slog.Warn("failed to load receipt items", "receipt_id", receipts[i].ID, "error", err)
			continue
		}
		receipts[i].Items = items
	}

	return receipts, nil
}

// GetReceipt retrieves a single receipt by ID with its items.
func (s *EtsyService) GetReceipt(ctx context.Context, id uuid.UUID) (*model.EtsyReceipt, error) {
	receipt, err := s.repo.GetReceiptByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if receipt == nil {
		return nil, nil
	}

	items, err := s.repo.GetReceiptItems(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("loading receipt items: %w", err)
	}
	receipt.Items = items

	return receipt, nil
}

// ProcessReceipt matches a receipt to templates and creates a unified order.
func (s *EtsyService) ProcessReceipt(ctx context.Context, id uuid.UUID, projectSvc *ProjectService, orderSvc *OrderService) (*model.Order, error) {
	receipt, err := s.GetReceipt(ctx, id)
	if err != nil {
		return nil, err
	}
	if receipt == nil {
		return nil, fmt.Errorf("receipt not found")
	}
	if receipt.IsProcessed {
		return nil, fmt.Errorf("receipt already processed")
	}

	// Build order items from receipt items
	var orderItems []model.OrderItem
	for _, item := range receipt.Items {
		orderItem := model.OrderItem{
			SKU:      item.SKU,
			Quantity: item.Quantity,
		}

		// Match to project by SKU
		if item.SKU != "" && projectSvc != nil {
			project, err := projectSvc.GetBySKU(ctx, item.SKU)
			if err != nil {
				slog.Warn("error looking up project by SKU", "sku", item.SKU, "error", err)
			} else if project != nil {
				orderItem.ProjectID = &project.ID
			}
		}

		orderItems = append(orderItems, orderItem)
	}

	// Create unified order
	order, err := orderSvc.CreateFromExternalOrder(
		ctx,
		model.OrderSourceEtsy,
		fmt.Sprintf("%d", receipt.EtsyReceiptID),
		receipt.Name,
		receipt.BuyerEmail,
		orderItems,
	)
	if err != nil {
		return nil, fmt.Errorf("creating unified order: %w", err)
	}

	// Mark receipt as processed and link to order
	if err := s.repo.UpdateReceiptProcessed(ctx, id, nil); err != nil {
		slog.Warn("failed to mark receipt as processed", "receipt_id", id, "error", err)
	}

	slog.Info("processed Etsy receipt to unified order", "receipt_id", receipt.EtsyReceiptID, "order_id", order.ID)
	return order, nil
}

// ---- Listing Methods ----

// SyncListings fetches active listings from Etsy and stores them locally.
func (s *EtsyService) SyncListings(ctx context.Context) (*model.SyncResult, error) {
	client, token, err := s.GetAuthenticatedClient(ctx)
	if err != nil {
		return nil, err
	}

	integration, err := s.repo.GetIntegration(ctx)
	if err != nil {
		return nil, err
	}

	opts := etsy.ListingQueryOptions{
		State: "active",
		Limit: 100,
	}

	listings, err := client.GetActiveListings(ctx, token, integration.ShopID, opts)
	if err != nil {
		return nil, fmt.Errorf("fetching listings: %w", err)
	}

	result := &model.SyncResult{
		TotalFetched: len(listings),
	}

	for _, apiListing := range listings {
		existing, err := s.repo.GetListingByEtsyID(ctx, apiListing.ListingID)
		if err != nil {
			slog.Error("error checking existing listing", "listing_id", apiListing.ListingID, "error", err)
			result.Errors++
			continue
		}

		listing := convertAPIListingToModel(apiListing)

		if existing != nil {
			listing.ID = existing.ID
			listing.CreatedAt = existing.CreatedAt
			result.Updated++
		} else {
			result.Created++
		}

		if err := s.repo.SaveListing(ctx, listing); err != nil {
			slog.Error("error saving listing", "listing_id", apiListing.ListingID, "error", err)
			result.Errors++
			continue
		}
	}

	// Update sync state
	now := time.Now()
	syncState, _ := s.repo.GetSyncState(ctx, integration.ShopID)
	if syncState == nil {
		syncState = &model.EtsySyncState{
			ShopID: integration.ShopID,
		}
	}
	syncState.LastListingSyncAt = &now
	if err := s.repo.SaveSyncState(ctx, syncState); err != nil {
		slog.Warn("failed to update sync state", "error", err)
	}

	slog.Info("listing sync completed", "fetched", result.TotalFetched, "created", result.Created, "updated", result.Updated)
	return result, nil
}

// convertAPIListingToModel converts an API listing to a model listing.
func convertAPIListingToModel(api etsy.APIListing) *model.EtsyListing {
	listing := &model.EtsyListing{
		EtsyListingID:    api.ListingID,
		EtsyShopID:       api.ShopID,
		Title:            api.Title,
		Description:      api.Description,
		State:            api.State,
		Quantity:         api.Quantity,
		URL:              api.URL,
		Views:            api.Views,
		NumFavorers:      api.NumFavorers,
		IsCustomizable:   api.IsCustomizable,
		IsPersonalizable: api.IsPersonalizable,
		HasVariations:    api.HasVariations,
		PriceCents:       etsy.MoneyToCents(api.Price),
		Currency:         api.Price.CurrencyCode,
		SyncedAt:         time.Now(),
	}

	if len(api.Tags) > 0 {
		if data, err := json.Marshal(api.Tags); err == nil {
			listing.Tags = data
		}
	}

	if len(api.SKUs) > 0 {
		if data, err := json.Marshal(api.SKUs); err == nil {
			listing.SKUs = data
		}
	}

	return listing
}

// ListListings retrieves stored listings with optional filtering.
func (s *EtsyService) ListListings(ctx context.Context, state string, limit, offset int) ([]model.EtsyListing, error) {
	return s.repo.ListListings(ctx, state, limit, offset)
}

// GetListing retrieves a single listing by ID.
func (s *EtsyService) GetListing(ctx context.Context, id uuid.UUID) (*model.EtsyListing, error) {
	return s.repo.GetListingByID(ctx, id)
}

// LinkListingToProject creates a mapping between a listing and a project.
func (s *EtsyService) LinkListingToProject(ctx context.Context, listingID uuid.UUID, projectID uuid.UUID, sku string, syncInventory bool) error {
	listing, err := s.repo.GetListingByID(ctx, listingID)
	if err != nil {
		return err
	}
	if listing == nil {
		return fmt.Errorf("listing not found")
	}

	link := &model.EtsyListingProject{
		EtsyListingID: listing.EtsyListingID,
		ProjectID:     projectID,
		SKU:           sku,
		SyncInventory: syncInventory,
	}

	return s.repo.SaveListingTemplate(ctx, link)
}

// UnlinkListingFromProject removes a mapping between a listing and a project.
func (s *EtsyService) UnlinkListingFromProject(ctx context.Context, listingID uuid.UUID, projectID uuid.UUID) error {
	listing, err := s.repo.GetListingByID(ctx, listingID)
	if err != nil {
		return err
	}
	if listing == nil {
		return fmt.Errorf("listing not found")
	}

	return s.repo.DeleteListingTemplate(ctx, listing.EtsyListingID, projectID)
}

func (s *EtsyService) LinkListingToTemplate(ctx context.Context, listingID uuid.UUID, templateID uuid.UUID, sku string, syncInventory bool) error {
	return s.LinkListingToProject(ctx, listingID, templateID, sku, syncInventory)
}

func (s *EtsyService) UnlinkListingFromTemplate(ctx context.Context, listingID uuid.UUID, templateID uuid.UUID) error {
	return s.UnlinkListingFromProject(ctx, listingID, templateID)
}

// SyncInventoryToEtsy pushes local inventory to Etsy for a listing.
func (s *EtsyService) SyncInventoryToEtsy(ctx context.Context, listingID uuid.UUID) error {
	client, token, err := s.GetAuthenticatedClient(ctx)
	if err != nil {
		return err
	}

	listing, err := s.repo.GetListingByID(ctx, listingID)
	if err != nil {
		return err
	}
	if listing == nil {
		return fmt.Errorf("listing not found")
	}

	// Get current inventory from Etsy
	inventory, err := client.GetListingInventory(ctx, token, listing.EtsyListingID)
	if err != nil {
		return fmt.Errorf("getting inventory: %w", err)
	}

	// For now, just log what we'd update - actual quantity calculation would
	// depend on template/spool inventory logic
	slog.Info("would sync inventory", "listing_id", listing.EtsyListingID, "products", len(inventory.Products))

	return nil
}

// ---- Webhook Methods ----

// SaveWebhookEvent saves an incoming webhook event.
func (s *EtsyService) SaveWebhookEvent(ctx context.Context, event *model.EtsyWebhookEvent) error {
	return s.repo.SaveWebhookEvent(ctx, event)
}

// HandleReceiptCreated handles a receipt.created webhook event.
func (s *EtsyService) HandleReceiptCreated(ctx context.Context, event *model.EtsyWebhookEvent) error {
	// Trigger a receipt sync to fetch the new receipt
	_, err := s.SyncReceipts(ctx)
	if err != nil {
		return fmt.Errorf("syncing receipts: %w", err)
	}

	// Mark event as processed
	return s.repo.UpdateWebhookEventProcessed(ctx, event.ID, "")
}

// HandleListingUpdated handles a listing.updated webhook event.
func (s *EtsyService) HandleListingUpdated(ctx context.Context, event *model.EtsyWebhookEvent) error {
	// Trigger a listing sync to update the listing
	_, err := s.SyncListings(ctx)
	if err != nil {
		return fmt.Errorf("syncing listings: %w", err)
	}

	return s.repo.UpdateWebhookEventProcessed(ctx, event.ID, "")
}

// HandleInventoryUpdated handles a listing.inventory.updated webhook event.
func (s *EtsyService) HandleInventoryUpdated(ctx context.Context, event *model.EtsyWebhookEvent) error {
	// For now, just mark as processed - could trigger inventory sync
	return s.repo.UpdateWebhookEventProcessed(ctx, event.ID, "")
}

// ListWebhookEvents retrieves webhook events with optional filtering.
func (s *EtsyService) ListWebhookEvents(ctx context.Context, eventType string, limit, offset int) ([]model.EtsyWebhookEvent, error) {
	return s.repo.ListWebhookEvents(ctx, eventType, limit, offset)
}

// ReprocessWebhookEvent retries processing a failed webhook event.
func (s *EtsyService) ReprocessWebhookEvent(ctx context.Context, eventID uuid.UUID) error {
	event, err := s.repo.GetWebhookEventByID(ctx, eventID)
	if err != nil {
		return err
	}
	if event == nil {
		return fmt.Errorf("event not found")
	}

	var processErr error
	switch event.EventType {
	case model.EtsyEventReceiptCreated, model.EtsyEventReceiptUpdated:
		processErr = s.HandleReceiptCreated(ctx, event)
	case model.EtsyEventListingUpdated:
		processErr = s.HandleListingUpdated(ctx, event)
	case model.EtsyEventListingInventoryUpdated:
		processErr = s.HandleInventoryUpdated(ctx, event)
	default:
		processErr = fmt.Errorf("unknown event type: %s", event.EventType)
	}

	if processErr != nil {
		s.repo.UpdateWebhookEventProcessed(ctx, eventID, processErr.Error())
		return processErr
	}

	return nil
}
