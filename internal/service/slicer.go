package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/repository"
	"github.com/Brook-sys/picofarm/internal/storage"
	"github.com/google/uuid"
)

type SlicerService struct {
	settings *SettingsService
	stls     *repository.STLLibraryRepository
	files    *repository.FileRepository
	gcodes   *GCodeLibraryService
	storage  storage.Storage
	// Three clients instead of one, because the operations they serve need
	// wildly different deadlines (see the timeouts below). A single 60 minute
	// client meant that a slicer that never answered /health kept the UI
	// waiting for an hour.
	quickClient   *http.Client
	profileClient *http.Client
	sliceClient   *http.Client
}

type SlicerConfig struct {
	ConnectionURL       string            `json:"connection_url"`
	ConnectionURLSource string            `json:"connection_url_source"`
	DefaultProfiles     map[string]string `json:"default_profiles"`
}

type SlicerDefaultProfileRequest struct {
	Category string `json:"category"`
	Name     string `json:"name"`
}

type SlicerHealth map[string]any
type SlicerProfileInfo map[string]any
type SlicerStatus map[string]any

type SlicerImportRequest struct {
	Category  string `json:"category"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	Overwrite bool   `json:"overwrite"`
}

type SlicerUploadProfileRequest struct {
	Category string `json:"category"`
	Name     string `json:"name"`
	JSON     string `json:"json"`
}

type SlicerSliceRequest struct {
	STLFileID             uuid.UUID                 `json:"stl_file_id"`
	Printer               string                    `json:"printer"`
	Preset                string                    `json:"preset"`
	Filament              string                    `json:"filament"`
	Arrange               bool                      `json:"arrange"`
	Orient                bool                      `json:"orient"`
	EnableSupport         bool                      `json:"enable_support"`
	BrimType              bool                      `json:"brim_type"`
	PrintSequenceByObject bool                      `json:"print_sequence_by_object"`
	ExportType            string                    `json:"export_type"`
	MulticolorOnePlate    bool                      `json:"multicolor_one_plate"`
	Overrides             map[string]map[string]any `json:"overrides"`
	DisplayName           string                    `json:"display_name"`
	SetDefault            bool                      `json:"set_default"`
}

type SlicerSliceResult struct {
	GCode              *model.GCodeLibraryFile `json:"gcode"`
	PrintTimeSeconds   string                  `json:"print_time_seconds,omitempty"`
	FilamentUsedGrams  string                  `json:"filament_used_g,omitempty"`
	FilamentUsedMM     string                  `json:"filament_used_mm,omitempty"`
	ContentDisposition string                  `json:"content_disposition,omitempty"`
}

type SlicerPreviewRequest struct {
	STLFileID             uuid.UUID `json:"stl_file_id"`
	Printer               string    `json:"printer"`
	Preset                string    `json:"preset"`
	Filament              string    `json:"filament"`
	Arrange               bool      `json:"arrange"`
	Orient                bool      `json:"orient"`
	EnableSupport         bool      `json:"enable_support"`
	BrimType              bool      `json:"brim_type"`
	PrintSequenceByObject bool      `json:"print_sequence_by_object"`
	MulticolorOnePlate    bool      `json:"multicolor_one_plate"`
}

type SlicerPreviewResult struct {
	UsesSupport    bool    `json:"usesSupport"`
	PrintTime      float64 `json:"printTime"`
	FilamentUsedG  float64 `json:"filamentUsedG"`
	FilamentUsedMm float64 `json:"filamentUsedMm"`
	Thumbnail      string  `json:"thumbnail"`
}

var slicerProfileNameInvalidChars = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

// Slicer endpoints have very different response times, so each class of call
// gets its own deadline:
//
//   - quick: health, slice status, listing a profile category, reading one
//     profile, resolving profile names. The UI calls these on page load and
//     while filling in the slice form; a slicer that is down must fail in
//     seconds, not hold the request for an hour.
//   - profile: writing profiles, including importing one from a remote URL,
//     which is a download the slicer performs on our behalf.
//   - slice: slicing a large STL really can take tens of minutes, so the
//     deadline stays generous.
const (
	slicerTimeoutQuick   = 5 * time.Second
	slicerTimeoutProfile = 60 * time.Second
	slicerTimeoutSlice   = 60 * time.Minute
)

// SlicerConnectionURLEnv points at the orca-slicer-api container. It overrides
// the stored setting so a deployment can pin the address (docker-compose,
// systemd, Kubernetes env) without depending on whatever an earlier version
// wrote into the database.
const SlicerConnectionURLEnv = "SLICER_CONNECTION_URL"

func NewSlicerService(settings *SettingsService, repos *repository.Repositories, store storage.Storage, gcodes *GCodeLibraryService) *SlicerService {
	return &SlicerService{
		settings:      settings,
		stls:          repos.STLLibrary,
		files:         repos.Files,
		gcodes:        gcodes,
		storage:       store,
		quickClient:   &http.Client{Timeout: slicerTimeoutQuick},
		profileClient: &http.Client{Timeout: slicerTimeoutProfile},
		sliceClient:   &http.Client{Timeout: slicerTimeoutSlice},
	}
}

// GetConfig returns the slicer configuration the UI needs, including the
// effective connection URL and which source won.
func (s *SlicerService) GetConfig(ctx context.Context) (SlicerConfig, error) {
	cfg := SlicerConfig{DefaultProfiles: map[string]string{}}
	urlValue, source, err := s.effectiveConnectionURL(ctx)
	if err != nil {
		return SlicerConfig{}, err
	}
	cfg.ConnectionURL = urlValue
	cfg.ConnectionURLSource = source
	for _, category := range []string{"printers", "presets", "filaments"} {
		defaultSetting, err := s.settings.Get(ctx, slicerDefaultProfileKey(category))
		if err == nil && defaultSetting != nil {
			cfg.DefaultProfiles[category] = defaultSetting.Value
		}
	}
	return cfg, nil
}

// effectiveConnectionURL resolves the connection URL: the environment wins over
// the stored setting, which in turn wins over nothing. The second return value
// names the source so the UI can explain why an edit appears to have no effect.
func (s *SlicerService) effectiveConnectionURL(ctx context.Context) (string, string, error) {
	if fromEnv := strings.TrimRight(strings.TrimSpace(os.Getenv(SlicerConnectionURLEnv)), "/"); fromEnv != "" {
		// A malformed env value is an operator error worth reporting instead of
		// silently falling back to a stale setting.
		parsed, err := url.Parse(fromEnv)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return "", "", fmt.Errorf("%s is not a valid slicer connection URL: %q", SlicerConnectionURLEnv, fromEnv)
		}
		return fromEnv, "env", nil
	}
	setting, err := s.settings.Get(ctx, "slicer_connection_url")
	if err != nil {
		return "", "", err
	}
	if setting == nil {
		return "", "", nil
	}
	return strings.TrimRight(strings.TrimSpace(setting.Value), "/"), "settings", nil
}

// SetConfig stores the slicer configuration and returns the configuration that
// is now in effect. It returns the effective URL rather than the stored one
// because those differ whenever the environment overrides the setting; echoing
// the request back would tell the operator their edit took effect when it did
// not.
//
// The connection URL is stored even when the environment currently overrides
// it, so removing the env var brings the stored value back instead of an empty
// configuration.
func (s *SlicerService) SetConfig(ctx context.Context, cfg SlicerConfig) (SlicerConfig, error) {
	cfg.ConnectionURL = strings.TrimRight(strings.TrimSpace(cfg.ConnectionURL), "/")
	if cfg.ConnectionURL != "" {
		parsed, err := url.Parse(cfg.ConnectionURL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return SlicerConfig{}, fmt.Errorf("invalid slicer connection URL")
		}
	}
	if err := s.settings.Set(ctx, "slicer_connection_url", cfg.ConnectionURL); err != nil {
		return SlicerConfig{}, err
	}
	return s.GetConfig(ctx)
}

func (s *SlicerService) Health(ctx context.Context) (SlicerHealth, error) {
	body, err := s.do(ctx, http.MethodGet, "/health", nil, "")
	if err != nil {
		return nil, err
	}
	var health SlicerHealth
	if err := json.Unmarshal(body, &health); err != nil {
		return nil, err
	}
	return health, nil
}

func (s *SlicerService) Status(ctx context.Context) (SlicerStatus, error) {
	body, err := s.do(ctx, http.MethodGet, "/slice/status", nil, "")
	if err != nil {
		return nil, err
	}
	var status SlicerStatus
	if err := json.Unmarshal(body, &status); err != nil {
		return nil, err
	}
	return status, nil
}

func (s *SlicerService) ListProfiles(ctx context.Context, category string) ([]SlicerProfileInfo, error) {
	category = normalizeSlicerCategory(category)
	body, err := s.do(ctx, http.MethodGet, "/profiles/"+category, nil, "")
	if err != nil {
		return nil, err
	}
	var profiles []SlicerProfileInfo
	if err := json.Unmarshal(body, &profiles); err != nil {
		return nil, err
	}
	defaultName := ""
	setting, err := s.settings.Get(ctx, slicerDefaultProfileKey(category))
	if err == nil && setting != nil {
		defaultName = setting.Value
	}
	if defaultName == "" && len(profiles) == 1 {
		if name, ok := profiles[0]["name"].(string); ok && strings.TrimSpace(name) != "" {
			defaultName = name
			_ = s.settings.Set(ctx, slicerDefaultProfileKey(category), defaultName)
		}
	}
	for _, profile := range profiles {
		if name, ok := profile["name"].(string); ok && name == defaultName {
			profile["default"] = true
		}
	}
	return profiles, nil
}

func (s *SlicerService) SetDefaultProfile(ctx context.Context, req SlicerDefaultProfileRequest) error {
	category := normalizeSlicerCategory(req.Category)
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return fmt.Errorf("profile name is required")
	}
	return s.settings.Set(ctx, slicerDefaultProfileKey(category), name)
}

func (s *SlicerService) GetProfileJSON(ctx context.Context, category, name string) (json.RawMessage, error) {
	category = normalizeSlicerCategory(category)
	body, err := s.do(ctx, http.MethodGet, "/profiles/"+category+"/"+url.PathEscape(name), nil, "")
	if err != nil {
		return nil, err
	}
	var raw json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func (s *SlicerService) ImportProfile(ctx context.Context, req SlicerImportRequest) (map[string]any, error) {
	category := normalizeSlicerCategory(req.Category)
	payload, _ := json.Marshal(map[string]any{"name": req.Name, "url": req.URL, "overwrite": req.Overwrite})
	body, err := s.doWithKind(ctx, slicerCallProfile, http.MethodPost, "/profiles/"+category+"/import-url", bytes.NewReader(payload), "application/json")
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *SlicerService) UploadProfileJSON(ctx context.Context, req SlicerUploadProfileRequest) (map[string]any, error) {
	category := normalizeSlicerCategory(req.Category)
	var raw map[string]any
	if err := json.Unmarshal([]byte(req.JSON), &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON profile")
	}
	profileName := strings.TrimSpace(req.Name)
	if profileName == "" {
		if rawName, ok := raw["name"].(string); ok {
			profileName = strings.TrimSpace(rawName)
		}
	}
	if profileName == "" {
		return nil, fmt.Errorf("profile name is required or JSON must contain a name field")
	}
	storedName := normalizeSlicerProfileName(profileName)
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("name", storedName)
	part, err := writer.CreateFormFile("file", storedName+".json")
	if err != nil {
		return nil, err
	}
	if _, err := part.Write([]byte(req.JSON)); err != nil {
		return nil, err
	}
	writer.Close()
	resp, err := s.doRawWithKind(ctx, slicerCallProfile, http.MethodPost, "/profiles/"+category+"/upload", body, writer.FormDataContentType())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("slicer error: %d %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}
	result["displayName"] = profileName
	result["storedName"] = storedName
	return result, nil
}

func (s *SlicerService) UpdateProfileFromSource(ctx context.Context, category, name string) (map[string]any, error) {
	category = normalizeSlicerCategory(category)
	body, err := s.doWithKind(ctx, slicerCallProfile, http.MethodPost, "/profiles/"+category+"/"+url.PathEscape(name)+"/update-from-source", nil, "")
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *SlicerService) ResolveProfiles(ctx context.Context, payload map[string]any) (map[string]any, error) {
	bodyBytes, _ := json.Marshal(payload)
	body, err := s.do(ctx, http.MethodPost, "/slice/resolve-profiles", bytes.NewReader(bodyBytes), "application/json")
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *SlicerService) SliceSTL(ctx context.Context, req SlicerSliceRequest) (*SlicerSliceResult, error) {
	stl, err := s.stls.GetByID(ctx, req.STLFileID)
	if err != nil || stl == nil {
		return nil, fmt.Errorf("stl file not found")
	}
	file, err := s.files.GetByID(ctx, stl.FileID)
	if err != nil || file == nil {
		return nil, fmt.Errorf("stl source file not found")
	}
	reader, err := s.storage.Get(file.StoragePath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", file.OriginalName)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, reader); err != nil {
		return nil, err
	}
	addFormField(writer, "printer", req.Printer)
	addFormField(writer, "preset", req.Preset)
	addFormField(writer, "filament", req.Filament)
	addFormField(writer, "exportType", firstNonEmpty(req.ExportType, "gcode"))
	addFormField(writer, "resolveProfiles", "true")
	addFormField(writer, "sanitizeProfiles", "true")
	if req.Arrange {
		addFormField(writer, "arrange", "true")
	}
	if req.Orient {
		addFormField(writer, "orient", "true")
	}
	if req.EnableSupport {
		addFormField(writer, "enableSupport", "true")
	}
	if req.BrimType {
		addFormField(writer, "brim_type", "true")
	}
	if req.PrintSequenceByObject {
		addFormField(writer, "print_sequence_by_object", "true")
	}
	if req.MulticolorOnePlate {
		addFormField(writer, "multicolorOnePlate", "true")
	}
	if len(req.Overrides) > 0 {
		overrides, _ := json.Marshal(req.Overrides)
		addFormField(writer, "overrides", string(overrides))
	}
	writer.Close()

	resp, err := s.doRawWithKind(ctx, slicerCallSlice, http.MethodPost, "/slice", body, writer.FormDataContentType())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("slicer error: %d %s", resp.StatusCode, strings.TrimSpace(string(errBody)))
	}
	if strings.Contains(resp.Header.Get("Content-Type"), "zip") {
		return nil, fmt.Errorf("slicer returned multiple files as zip; choose a single plate or export gcode")
	}
	filename := req.DisplayName
	if strings.TrimSpace(filename) == "" {
		base := strings.TrimSuffix(file.OriginalName, filepath.Ext(file.OriginalName))
		filename = base + ".gcode"
	}
	if !strings.HasSuffix(strings.ToLower(filename), ".gcode") {
		filename += ".gcode"
	}
	gcode, err := s.gcodes.UploadWithParent(ctx, filename, resp.Body, &stl.ID)
	if err != nil {
		return nil, err
	}
	if req.SetDefault {
		_ = s.gcodes.SetDefaultForSTL(ctx, gcode.ID)
	}
	return &SlicerSliceResult{GCode: gcode, PrintTimeSeconds: resp.Header.Get("X-Print-Time-Seconds"), FilamentUsedGrams: resp.Header.Get("X-Filament-Used-g"), FilamentUsedMM: resp.Header.Get("X-Filament-Used-mm"), ContentDisposition: resp.Header.Get("Content-Disposition")}, nil
}

func (s *SlicerService) PreviewSTL(ctx context.Context, req SlicerPreviewRequest) (*SlicerPreviewResult, error) {
	stl, err := s.stls.GetByID(ctx, req.STLFileID)
	if err != nil || stl == nil {
		return nil, fmt.Errorf("stl file not found")
	}
	file, err := s.files.GetByID(ctx, stl.FileID)
	if err != nil || file == nil {
		return nil, fmt.Errorf("stl source file not found")
	}
	reader, err := s.storage.Get(file.StoragePath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", file.OriginalName)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, reader); err != nil {
		return nil, err
	}
	addFormField(writer, "printer", req.Printer)
	addFormField(writer, "preset", req.Preset)
	addFormField(writer, "filament", req.Filament)
	addFormField(writer, "resolveProfiles", "true")
	addFormField(writer, "sanitizeProfiles", "true")
	if req.Arrange {
		addFormField(writer, "arrange", "true")
	}
	if req.Orient {
		addFormField(writer, "orient", "true")
	}
	if req.EnableSupport {
		addFormField(writer, "enableSupport", "true")
	}
	if req.BrimType {
		addFormField(writer, "brim_type", "true")
	}
	if req.PrintSequenceByObject {
		addFormField(writer, "print_sequence_by_object", "true")
	}
	if req.MulticolorOnePlate {
		addFormField(writer, "multicolorOnePlate", "true")
	}
	writer.Close()

	resp, err := s.doRawWithKind(ctx, slicerCallSlice, http.MethodPost, "/slice/preview", body, writer.FormDataContentType())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("slicer error: %d %s", resp.StatusCode, strings.TrimSpace(string(errBody)))
	}
	var result SlicerPreviewResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("invalid preview response")
	}
	return &result, nil
}

// slicerCallKind selects the HTTP deadline for a call. Slicing is slow by
// nature; everything else must fail fast or the UI hangs.
type slicerCallKind int

const (
	slicerCallQuick slicerCallKind = iota
	slicerCallProfile
	slicerCallSlice
)

// do performs a call with the quick deadline. Anything that the UI waits on
// while it loads or while the operator fills in the slice form goes through
// here.
func (s *SlicerService) do(ctx context.Context, method, path string, body io.Reader, contentType string) ([]byte, error) {
	return s.doWithKind(ctx, slicerCallQuick, method, path, body, contentType)
}

// doWithKind is the short-deadline path used by the profile and health calls.
func (s *SlicerService) doWithKind(ctx context.Context, kind slicerCallKind, method, path string, body io.Reader, contentType string) ([]byte, error) {
	resp, err := s.doRawWithKind(ctx, kind, method, path, body, contentType)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("slicer error: %d %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return respBody, nil
}

func (s *SlicerService) doRawWithKind(ctx context.Context, kind slicerCallKind, method, path string, body io.Reader, contentType string) (*http.Response, error) {
	// Only the connection URL is read here. Reading the whole config meant three
	// extra settings lookups per call (one per default profile) that nothing on
	// the request path uses.
	baseURL, _, err := s.effectiveConnectionURL(ctx)
	if err != nil {
		return nil, err
	}
	if baseURL == "" {
		return nil, fmt.Errorf("slicer connection URL is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	switch kind {
	case slicerCallProfile:
		return s.profileClient.Do(req)
	case slicerCallSlice:
		return s.sliceClient.Do(req)
	default:
		return s.quickClient.Do(req)
	}
}

func slicerDefaultProfileKey(category string) string {
	return "slicer_default_" + normalizeSlicerCategory(category)
}

func normalizeSlicerProfileName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, ".json", "")
	normalized := slicerProfileNameInvalidChars.ReplaceAllString(name, "_")
	normalized = strings.Trim(normalized, "_-")
	if normalized == "" {
		return "profile"
	}
	return normalized
}

func normalizeSlicerCategory(category string) string {
	switch category {
	case "printer", "printers":
		return "printers"
	case "preset", "presets":
		return "presets"
	case "filament", "filaments":
		return "filaments"
	default:
		return category
	}
}

func addFormField(writer *multipart.Writer, key string, value string) {
	if strings.TrimSpace(value) != "" {
		_ = writer.WriteField(key, value)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
