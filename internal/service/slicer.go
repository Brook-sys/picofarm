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
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/repository"
	"github.com/Brook-sys/picofarm/internal/storage"
)

type SlicerService struct {
	settings *SettingsService
	stls     *repository.STLLibraryRepository
	files    *repository.FileRepository
	gcodes   *GCodeLibraryService
	storage  storage.Storage
	client   *http.Client
}

type SlicerConfig struct {
	ConnectionURL string `json:"connection_url"`
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
	STLFileID          uuid.UUID                 `json:"stl_file_id"`
	Printer            string                    `json:"printer"`
	Preset             string                    `json:"preset"`
	Filament           string                    `json:"filament"`
	Arrange            bool                      `json:"arrange"`
	Orient             bool                      `json:"orient"`
	EnableSupport      bool                      `json:"enable_support"`
	ExportType         string                    `json:"export_type"`
	MulticolorOnePlate bool                      `json:"multicolor_one_plate"`
	Overrides          map[string]map[string]any `json:"overrides"`
	DisplayName        string                    `json:"display_name"`
	SetDefault         bool                      `json:"set_default"`
}

type SlicerSliceResult struct {
	GCode              *model.GCodeLibraryFile `json:"gcode"`
	PrintTimeSeconds   string                  `json:"print_time_seconds,omitempty"`
	FilamentUsedGrams  string                  `json:"filament_used_g,omitempty"`
	FilamentUsedMM     string                  `json:"filament_used_mm,omitempty"`
	ContentDisposition string                  `json:"content_disposition,omitempty"`
}

var slicerProfileNameInvalidChars = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

func NewSlicerService(settings *SettingsService, repos *repository.Repositories, store storage.Storage, gcodes *GCodeLibraryService) *SlicerService {
	return &SlicerService{settings: settings, stls: repos.STLLibrary, files: repos.Files, gcodes: gcodes, storage: store, client: &http.Client{Timeout: 60 * time.Minute}}
}

func (s *SlicerService) GetConfig(ctx context.Context) (SlicerConfig, error) {
	setting, err := s.settings.Get(ctx, "slicer_connection_url")
	if err != nil || setting == nil {
		return SlicerConfig{}, err
	}
	return SlicerConfig{ConnectionURL: setting.Value}, nil
}

func (s *SlicerService) SetConfig(ctx context.Context, cfg SlicerConfig) error {
	cfg.ConnectionURL = strings.TrimRight(strings.TrimSpace(cfg.ConnectionURL), "/")
	if cfg.ConnectionURL != "" {
		parsed, err := url.Parse(cfg.ConnectionURL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return fmt.Errorf("invalid slicer connection URL")
		}
	}
	return s.settings.Set(ctx, "slicer_connection_url", cfg.ConnectionURL)
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
	return profiles, nil
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
	body, err := s.do(ctx, http.MethodPost, "/profiles/"+category+"/import-url", bytes.NewReader(payload), "application/json")
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
	resp, err := s.doRaw(ctx, http.MethodPost, "/profiles/"+category+"/upload", body, writer.FormDataContentType())
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
	body, err := s.do(ctx, http.MethodPost, "/profiles/"+category+"/"+url.PathEscape(name)+"/update-from-source", nil, "")
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
	if req.MulticolorOnePlate {
		addFormField(writer, "multicolorOnePlate", "true")
	}
	if len(req.Overrides) > 0 {
		overrides, _ := json.Marshal(req.Overrides)
		addFormField(writer, "overrides", string(overrides))
	}
	writer.Close()

	resp, err := s.doRaw(ctx, http.MethodPost, "/slice", body, writer.FormDataContentType())
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

func (s *SlicerService) do(ctx context.Context, method, path string, body io.Reader, contentType string) ([]byte, error) {
	resp, err := s.doRaw(ctx, method, path, body, contentType)
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

func (s *SlicerService) doRaw(ctx context.Context, method, path string, body io.Reader, contentType string) (*http.Response, error) {
	cfg, err := s.GetConfig(ctx)
	if err != nil {
		return nil, err
	}
	if cfg.ConnectionURL == "" {
		return nil, fmt.Errorf("slicer connection URL is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(cfg.ConnectionURL, "/")+path, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return s.client.Do(req)
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
