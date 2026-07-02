package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type ThingiverseImportService struct {
	settings *SettingsService
	stls     *STLLibraryService
	client   *http.Client
}

type ThingiverseResolvedModel struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Creator     string `json:"creator"`
	Description string `json:"description"`
	License     string `json:"license"`
	Thumbnail   string `json:"thumbnail"`
	SourceURL   string `json:"source_url"`
}

type ThingiverseFile struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	DownloadURL string `json:"download_url"`
}

type ThingiverseImportPreviewResult struct {
	SourceURL string                         `json:"source_url"`
	Title     string                         `json:"title"`
	Files     []ThingiverseImportPreviewFile `json:"files"`
	TotalSTL  int                            `json:"total_stl"`
	Selected  int                            `json:"selected"`
}

type ThingiverseImportPreviewFile struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	DownloadURL string `json:"download_url"`
	Selected    bool   `json:"selected"`
	Exists      bool   `json:"exists"`
}

func NewThingiverseImportService(settings *SettingsService, stls *STLLibraryService) *ThingiverseImportService {
	return &ThingiverseImportService{
		settings: settings,
		stls:     stls,
		client:   &http.Client{Timeout: 120 * time.Second},
	}
}

func (s *ThingiverseImportService) getToken(ctx context.Context) (string, error) {
	if s.settings == nil {
		return "", fmt.Errorf("thingiverse api token not configured")
	}
	setting, err := s.settings.Get(ctx, "thingiverse_api_token")
	if err != nil || setting == nil || strings.TrimSpace(setting.Value) == "" {
		return "", fmt.Errorf("thingiverse api token not configured")
	}
	return setting.Value, nil
}

func (s *ThingiverseImportService) do(ctx context.Context, path string, token string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.thingiverse.com"+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "Picofarm/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("thingiverse api returned %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func (s *ThingiverseImportService) Resolve(ctx context.Context, rawURL string) (*ThingiverseResolvedModel, error) {
	token, err := s.getToken(ctx)
	if err != nil {
		return nil, err
	}

	id, err := extractThingID(rawURL)
	if err != nil {
		return nil, err
	}

	body, err := s.do(ctx, fmt.Sprintf("/things/%d", id), token)
	if err != nil {
		return nil, err
	}

	var thing struct {
		ID          int                   `json:"id"`
		Name        string                `json:"name"`
		Creator     struct{ Name string } `json:"creator"`
		Description string                `json:"description"`
		License     string                `json:"license"`
		Thumbnail   string                `json:"thumbnail"`
	}
	if err := json.Unmarshal(body, &thing); err != nil {
		return nil, fmt.Errorf("failed to parse thingiverse response")
	}

	return &ThingiverseResolvedModel{
		ID:          thing.ID,
		Name:        thing.Name,
		Creator:     thing.Creator.Name,
		Description: thing.Description,
		License:     thing.License,
		Thumbnail:   thing.Thumbnail,
		SourceURL:   rawURL,
	}, nil
}

func (s *ThingiverseImportService) Preview(ctx context.Context, rawURL string) (*ModelImportPreview, error) {
	resolved, err := s.Resolve(ctx, rawURL)
	if err != nil {
		return nil, err
	}

	token, err := s.getToken(ctx)
	if err != nil {
		return nil, err
	}

	// Get files
	body, err := s.do(ctx, fmt.Sprintf("/things/%d/files", resolved.ID), token)
	if err != nil {
		return nil, err
	}

	var files []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &files); err != nil {
		return nil, fmt.Errorf("failed to parse files")
	}

	var stlFiles []ModelImportFile
	for _, f := range files {
		if strings.HasSuffix(strings.ToLower(f.Name), ".stl") {
			stlFiles = append(stlFiles, ModelImportFile{
				Name: f.Name,
				URL:  fmt.Sprintf("https://api.thingiverse.com/files/%d/download", f.ID),
			})
		}
	}

	return &ModelImportPreview{
		Provider:    "Thingiverse",
		SourceURL:   resolved.SourceURL,
		Title:       resolved.Name,
		Description: resolved.Description,
		Author:      resolved.Creator,
		License:     resolved.License,
		ImageURL:    resolved.Thumbnail,
		STLFiles:    stlFiles,
	}, nil
}

func (s *ThingiverseImportService) Import(ctx context.Context, req ModelImportRequest) (*ModelImportResult, error) {
	resolved, err := s.Resolve(ctx, req.URL)
	if err != nil {
		return nil, err
	}

	token, err := s.getToken(ctx)
	if err != nil {
		return nil, err
	}

	// Get all files again to map names to download URLs
	body, err := s.do(ctx, fmt.Sprintf("/things/%d/files", resolved.ID), token)
	if err != nil {
		return nil, err
	}

	var apiFiles []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	json.Unmarshal(body, &apiFiles)

	selected := map[string]bool{}
	for _, u := range req.STLURLs {
		selected[u] = true
	}

	result := &ModelImportResult{}
	for _, f := range apiFiles {
		if !strings.HasSuffix(strings.ToLower(f.Name), ".stl") {
			continue
		}
		if len(selected) > 0 && !selected[f.Name] {
			continue
		}

		dlURL := fmt.Sprintf("https://api.thingiverse.com/files/%d/download", f.ID)
		reqDL, _ := http.NewRequestWithContext(ctx, http.MethodGet, dlURL, nil)
		reqDL.Header.Set("Authorization", "Bearer "+token)
		reqDL.Header.Set("User-Agent", "Picofarm/1.0")

		resp, err := s.client.Do(reqDL)
		if err != nil || resp.StatusCode >= 400 {
			continue
		}

		stl, err := s.stls.Upload(ctx, f.Name, resp.Body, nil)
		_ = resp.Body.Close()
		if err != nil {
			continue
		}
		result.STLs = append(result.STLs, *stl)
	}

	return result, nil
}

func extractThingID(rawURL string) (int, error) {
	re := regexp.MustCompile(`thing:(\d+)`)
	m := re.FindStringSubmatch(rawURL)
	if len(m) != 2 {
		return 0, fmt.Errorf("invalid thingiverse url")
	}
	id, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, fmt.Errorf("invalid thingiverse url")
	}
	return id, nil
}

func (s *ThingiverseImportService) ImportPreview(ctx context.Context, req ModelImportRequest) (*ThingiverseImportPreviewResult, error) {
	resolved, err := s.Resolve(ctx, req.URL)
	if err != nil {
		return nil, err
	}

	token, err := s.getToken(ctx)
	if err != nil {
		return nil, err
	}

	body, err := s.do(ctx, fmt.Sprintf("/things/%d/files", resolved.ID), token)
	if err != nil {
		return nil, err
	}

	var apiFiles []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &apiFiles); err != nil {
		return nil, fmt.Errorf("failed to parse files")
	}

	selected := map[string]bool{}
	for _, u := range req.STLURLs {
		selected[u] = true
	}

	result := &ThingiverseImportPreviewResult{
		SourceURL: resolved.SourceURL,
		Title:     resolved.Name,
		Files:     []ThingiverseImportPreviewFile{},
	}

	for _, f := range apiFiles {
		if !strings.HasSuffix(strings.ToLower(f.Name), ".stl") {
			continue
		}

		dlURL := fmt.Sprintf("https://api.thingiverse.com/files/%d/download", f.ID)
		sel := len(selected) == 0 || selected[f.Name]

		exists := false
		if s.stls != nil {
			// best effort: check if file with same name exists in stl library
			// we do not have a direct GetByName, so we skip expensive check for now
			// existence will be false unless we implement it later
			exists = false
		}

		result.Files = append(result.Files, ThingiverseImportPreviewFile{
			ID:          f.ID,
			Name:        f.Name,
			DownloadURL: dlURL,
			Selected:    sel,
			Exists:      exists,
		})

		if sel {
			result.Selected++
		}
		result.TotalSTL++
	}

	return result, nil
}
