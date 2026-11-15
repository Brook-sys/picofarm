package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/Brook-sys/picofarm/internal/repository"
)

const SlicerProfileJSONLimit = 1 << 20

var ErrSlicerProfileConflict = errors.New("profile changed since it was loaded; reload before saving")

// Serialize every profile mutation in this process, including editor saves,
// uploads, imports, bundle restores and source updates. Direct upstream edits
// and other PicoFarm processes remain outside this lock.
var slicerEditorMu sync.Mutex

type SlicerProfileValidation struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
}
type SlicerSaveProfileRequest struct {
	Category     string `json:"category"`
	Name         string `json:"name"`
	JSON         string `json:"json"`
	ExpectedHash string `json:"expected_hash"`
}
type SlicerProfileHistory struct {
	CurrentJSON string                             `json:"current_json"`
	CurrentHash string                             `json:"current_hash"`
	Revisions   []repository.SlicerProfileRevision `json:"revisions"`
}

// ValidateSlicerProfile performs bounded structural and common parameter checks.
// It is intentionally not an Orca schema validator or print-safety certification.
func ValidateSlicerProfile(req SlicerUploadProfileRequest) SlicerProfileValidation {
	v := SlicerProfileValidation{Errors: []string{}, Warnings: []string{"Checks cover structure and common limits only; Orca compatibility and print safety are not certified."}}
	category := normalizeSlicerCategory(strings.TrimSpace(req.Category))
	if category != "printers" && category != "presets" && category != "filaments" {
		v.Errors = append(v.Errors, "Unknown profile category")
	}
	if len(req.JSON) > SlicerProfileJSONLimit {
		v.Errors = append(v.Errors, "Profile JSON exceeds 1 MiB")
		return v
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(req.JSON), &raw); err != nil || raw == nil {
		v.Errors = append(v.Errors, "Profile JSON must be a non-null object")
		return v
	}
	if len(raw) == 0 {
		v.Errors = append(v.Errors, "Profile JSON must not be empty")
	}
	if kind, ok := raw["type"]; ok {
		expected := map[string]string{"printers": "machine", "presets": "process", "filaments": "filament"}[category]
		if s, ok := kind.(string); !ok || s != expected {
			v.Errors = append(v.Errors, "Profile type does not match its category")
		}
	}
	for _, key := range []string{"name", "inherits", "type"} {
		if value, ok := raw[key]; ok {
			if _, ok := value.(string); !ok {
				v.Errors = append(v.Errors, key+" must be a string")
			}
		}
	}
	if name, ok := raw["name"].(string); strings.TrimSpace(req.Name) == "" && (!ok || strings.TrimSpace(name) == "") {
		v.Errors = append(v.Errors, "Profile name is required")
	}
	if parent, ok := raw["inherits"].(string); ok && strings.TrimSpace(parent) != "" {
		if parent == req.Name || parent == raw["name"] {
			v.Errors = append(v.Errors, "Profile cannot inherit from itself")
		}
		v.Warnings = append(v.Warnings, "Inherited values are not checked here. The parent must exist and resolve in the target slicer.")
	}
	// Orca commonly encodes numbers as strings and filament values as arrays.
	limits := map[string][2]float64{"layer_height": {0.001, 10}, "initial_layer_print_height": {0.001, 10}, "nozzle_diameter": {0.05, 10}, "filament_diameter": {0.1, 10}, "nozzle_temperature": {0, 500}, "nozzle_temperature_initial_layer": {0, 500}, "hot_plate_temp": {0, 200}, "hot_plate_temp_initial_layer": {0, 200}, "bed_temperature": {0, 200}, "wall_loops": {0, 1000}, "sparse_infill_density": {0, 100}}
	for key, bounds := range limits {
		if value, ok := raw[key]; ok {
			values := []any{value}
			if array, ok := value.([]any); ok {
				values = array
				if len(values) == 0 || len(values) > 256 {
					v.Errors = append(v.Errors, key+" array must have 1–256 values")
					continue
				}
			}
			for _, item := range values {
				var n float64
				var err error
				switch value := item.(type) {
				case float64:
					n = value
				case string:
					n, err = strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(value), "%"), 64)
				default:
					err = fmt.Errorf("not numeric")
				}
				// -1 is an Orca automatic/inherited sentinel, not a physical value.
				if n == -1 && err == nil {
					v.Warnings = append(v.Warnings, key+": automatic/inherited (-1) value is not range-checked")
					continue
				}
				if err != nil || !(n >= bounds[0] && n <= bounds[1]) {
					v.Errors = append(v.Errors, fmt.Sprintf("%s must contain numbers between %g and %g", key, bounds[0], bounds[1]))
					break
				}
			}
		}
	}
	v.Valid = len(v.Errors) == 0
	return v
}
func (s *SlicerService) ValidateProfile(_ context.Context, req SlicerUploadProfileRequest) SlicerProfileValidation {
	return ValidateSlicerProfile(req)
}

func slicerSemanticHash(raw string) (string, error) {
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return "", err
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:]), nil
}
func (s *SlicerService) profileHistorySource(ctx context.Context) (string, error) {
	source, _, err := s.effectiveConnectionURL(ctx)
	if err != nil {
		return "", err
	}
	if source == "" {
		return "", fmt.Errorf("slicer connection URL is not configured")
	}
	hash := sha256.Sum256([]byte(source))
	return hex.EncodeToString(hash[:]), nil
}

// pinProfileOperation captures the effective destination once, without mutating
// the shared service or rereading settings between validation and network I/O.
func (s *SlicerService) pinProfileOperation(ctx context.Context) (*SlicerService, error) {
	endpoint, _, err := s.effectiveConnectionURL(ctx)
	if err != nil {
		return nil, err
	}
	if endpoint == "" {
		return nil, fmt.Errorf("slicer connection URL is not configured")
	}
	operation := *s
	operation.operationURL = endpoint
	return &operation, nil
}

func (s *SlicerService) ProfileHistory(ctx context.Context, category, name string) (*SlicerProfileHistory, error) {
	if s.operationURL == "" {
		operation, err := s.pinProfileOperation(ctx)
		if err != nil {
			return nil, err
		}
		return operation.ProfileHistory(ctx, category, name)
	}
	category = normalizeSlicerCategory(category)
	if _, err := resolveSlicerCategories([]string{category}); err != nil {
		return nil, err
	}
	source, err := s.profileHistorySource(ctx)
	if err != nil {
		return nil, err
	}
	revisions, err := s.settings.repo.ListSlicerRevisions(ctx, source, category, name)
	if err != nil {
		return nil, err
	}
	result := &SlicerProfileHistory{Revisions: revisions}
	raw, err := s.GetProfileJSON(ctx, category, name)
	if err != nil {
		return nil, err
	}
	result.CurrentJSON = string(raw)
	result.CurrentHash, err = slicerSemanticHash(string(raw))
	return result, err
}
func (s *SlicerService) SaveProfileRevision(ctx context.Context, req SlicerSaveProfileRequest) (*SlicerProfileHistory, error) {
	validation := ValidateSlicerProfile(SlicerUploadProfileRequest{Category: req.Category, Name: req.Name, JSON: req.JSON})
	if !validation.Valid {
		return nil, fmt.Errorf("invalid profile: %s", strings.Join(validation.Errors, "; "))
	}
	if req.ExpectedHash == "" {
		return nil, fmt.Errorf("expected_hash is required")
	}
	// Editing an existing upstream filename must not silently write another one.
	if normalizeSlicerProfileName(req.Name) != req.Name {
		return nil, fmt.Errorf("profile filename cannot be safely edited; export and reimport under a normalized name")
	}
	if !s.profileMutationLocked {
		slicerEditorMu.Lock()
		defer slicerEditorMu.Unlock()
		operation, err := s.pinProfileOperation(ctx)
		if err != nil {
			return nil, err
		}
		operation.profileMutationLocked = true
		return operation.SaveProfileRevision(ctx, req)
	}
	current, err := s.ProfileHistory(ctx, req.Category, req.Name)
	if err != nil {
		return nil, err
	}
	if current.CurrentHash != req.ExpectedHash {
		return nil, ErrSlicerProfileConflict
	}
	source, err := s.profileHistorySource(ctx)
	if err != nil {
		return nil, err
	}
	category := normalizeSlicerCategory(req.Category)
	// Persist BOTH the preimage and proposed revision before any network write.
	// Snapshots deliberately make no assertion about upload success: a timeout
	// may mean the slicer accepted the request but its response was lost.
	if err := s.settings.repo.AppendSlicerRevisions(ctx, source, category, req.Name, current.CurrentJSON, req.JSON); err != nil {
		return nil, err
	}
	if _, err := s.UploadProfileJSON(ctx, SlicerUploadProfileRequest{Category: category, Name: req.Name, JSON: req.JSON}); err != nil {
		return nil, fmt.Errorf("save not confirmed; recovery snapshots retained: %w", err)
	}
	return s.ProfileHistory(ctx, category, req.Name)
}
