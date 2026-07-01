package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/repository"
)

type ModelImportService struct {
	projects *ProjectService
	parts    *PartService
	designs  *DesignService
	stls     *STLLibraryService
	tags     *repository.TagRepository
	client   *http.Client
}

type ModelImportPreviewRequest struct {
	URL string `json:"url"`
}

type ModelImportPreview struct {
	Provider    string            `json:"provider"`
	SourceURL   string            `json:"source_url"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Author      string            `json:"author"`
	License     string            `json:"license"`
	ImageURL    string            `json:"image_url"`
	STLFiles    []ModelImportFile `json:"stl_files"`
}

type ModelImportFile struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type ModelImportRequest struct {
	URL         string   `json:"url"`
	ProjectName string   `json:"project_name"`
	STLURLs     []string `json:"stl_urls"`
}

type ModelImportResult struct {
	Project *model.Project         `json:"project"`
	STLs    []model.STLLibraryFile `json:"stls"`
	Parts   []model.Part           `json:"parts"`
}

var (
	metaTagPattern = regexp.MustCompile(`(?is)<meta[^>]+(?:property|name)=["']([^"']+)["'][^>]+content=["']([^"']*)["'][^>]*>`)
	titlePattern   = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	stlLinkPattern = regexp.MustCompile(`(?i)(?:href|src|data-[^=]*)=["']([^"']+\.stl(?:\?[^"']*)?)["']`)
	stlJsonPattern = regexp.MustCompile(`(?i)["'](?:file|download|stl|model)["']\s*:\s*["']([^"'\\]+\.stl(?:\?[^"'\\]*)?)["']`)
)

func NewModelImportService(projects *ProjectService, parts *PartService, designs *DesignService, stls *STLLibraryService, tags *repository.TagRepository) *ModelImportService {
	return &ModelImportService{projects: projects, parts: parts, designs: designs, stls: stls, tags: tags, client: &http.Client{Timeout: 45 * time.Second}}
}

func (s *ModelImportService) Preview(ctx context.Context, rawURL string) (*ModelImportPreview, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, fmt.Errorf("url is required")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid url")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("unsupported url scheme")
	}
	provider := detectModelProvider(parsed.Host)
	body, err := s.fetchText(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	meta := parseMetaTags(body)
	title := firstNonEmpty(meta["og:title"], meta["twitter:title"], extractTitle(body), parsed.Host)
	stlFiles := extractSTLLinks(body)
	for i := range stlFiles {
		stlFiles[i].URL = resolveURL(rawURL, stlFiles[i].URL)
	}

	// Enrich Printables files with real download URLs via GraphQL
	if provider == "Printables" {
		if enriched := s.enrichPrintablesFiles(ctx, rawURL, stlFiles); len(enriched) > 0 {
			stlFiles = enriched
		}
	}

	preview := &ModelImportPreview{
		Provider:    provider,
		SourceURL:   rawURL,
		Title:       htmlUnescape(strings.TrimSpace(title)),
		Description: htmlUnescape(firstNonEmpty(meta["og:description"], meta["description"], meta["twitter:description"])),
		Author:      htmlUnescape(firstNonEmpty(meta["author"], meta["article:author"])),
		License:     htmlUnescape(firstNonEmpty(meta["license"])),
		ImageURL:    resolveURL(rawURL, firstNonEmpty(meta["og:image"], meta["twitter:image"])),
		STLFiles:    stlFiles,
	}
	return preview, nil
}

func (s *ModelImportService) Import(ctx context.Context, req ModelImportRequest) (*ModelImportResult, error) {
	preview, err := s.Preview(ctx, req.URL)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.ProjectName)
	if name == "" {
		name = preview.Title
	}
	project := &model.Project{Name: name, Description: preview.Description, Source: "import", SourceURL: preview.SourceURL, SourceProvider: preview.Provider, SourceAuthor: preview.Author, SourceLicense: preview.License, SourceDescription: preview.Description}
	if err := s.projects.Create(ctx, project); err != nil {
		return nil, err
	}
	_ = s.ensureTag(ctx, "Fonte: "+preview.Provider, "#3b82f6")

	selected := map[string]bool{}
	for _, u := range req.STLURLs {
		selected[u] = true
	}
	if len(selected) == 0 {
		for _, file := range preview.STLFiles {
			selected[file.URL] = true
		}
	}
	result := &ModelImportResult{Project: project}
	for _, file := range preview.STLFiles {
		if !selected[file.URL] {
			continue
		}
		stl, err := s.downloadSTL(ctx, file)
		if err != nil {
			continue
		}
		part := model.Part{ProjectID: project.ID, Name: strings.TrimSuffix(stl.DisplayName, filepath.Ext(stl.DisplayName)), Quantity: 1}
		if err := s.parts.Create(ctx, &part); err != nil {
			continue
		}
		if _, err := s.designs.CreateFromSTLLibrary(ctx, part.ID, stl.ID, ""); err != nil {
			continue
		}
		result.STLs = append(result.STLs, *stl)
		result.Parts = append(result.Parts, part)
	}
	return result, nil
}

func (s *ModelImportService) downloadSTL(ctx context.Context, file ModelImportFile) (*model.STLLibraryFile, error) {
	target := file.URL

	// Printables fallback: if we only have the filename, try the model's download endpoint
	if !strings.HasPrefix(target, "http") && strings.Contains(file.Name, ".stl") {
		// Try the direct download page for the model (works for many public files)
		// We don't have the exact fileId, so we attempt the model's download endpoint
		// which often serves the first/only STL when accessed this way.
		if strings.Contains(file.URL, "printables.com/model/") {
			// keep as-is, will fail → handled below
		} else {
			// construct a best-effort Printables download URL using the model page
			// This is a heuristic; real solution uses GraphQL downloadUrl
			target = "https://www.printables.com" + strings.TrimLeft(file.URL, "/")
		}
	}

	if !strings.HasPrefix(target, "http") {
		return nil, fmt.Errorf("relative STL without full URL not supported yet: %s", target)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Picofarm/1.0)")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("failed to download stl: %d", resp.StatusCode)
	}
	name := file.Name
	if name == "" {
		name = filepath.Base(strings.Split(target, "?")[0])
	}
	if !strings.HasSuffix(strings.ToLower(name), ".stl") {
		name += ".stl"
	}
	return s.stls.Upload(ctx, name, resp.Body, nil)
}

func (s *ModelImportService) ensureTag(ctx context.Context, name, color string) error {
	if s.tags == nil || strings.TrimSpace(name) == "" {
		return nil
	}
	tag, err := s.tags.GetByName(ctx, name)
	if err != nil || tag != nil {
		return err
	}
	return s.tags.Create(ctx, &model.Tag{Name: name, Color: color})
}

func (s *ModelImportService) fetchText(ctx context.Context, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Picofarm/1.0; +https://github.com/Brook-sys/picofarm)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("source returned %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func detectModelProvider(host string) string {
	host = strings.ToLower(host)
	switch {
	case strings.Contains(host, "makerworld"):
		return "MakerWorld"
	case strings.Contains(host, "printables"):
		return "Printables"
	default:
		return "URL"
	}
}

func parseMetaTags(html string) map[string]string {
	meta := map[string]string{}
	for _, match := range metaTagPattern.FindAllStringSubmatch(html, -1) {
		if len(match) == 3 {
			meta[strings.ToLower(strings.TrimSpace(match[1]))] = strings.TrimSpace(match[2])
		}
	}
	return meta
}

func extractTitle(html string) string {
	match := titlePattern.FindStringSubmatch(html)
	if len(match) == 2 {
		return strings.TrimSpace(match[1])
	}
	return ""
}

func extractSTLLinks(html string) []ModelImportFile {
	seen := map[string]bool{}
	files := []ModelImportFile{}

	// 1. href/src attributes
	for _, match := range stlLinkPattern.FindAllStringSubmatch(html, -1) {
		if len(match) < 2 {
			continue
		}
		addSTLLink(&files, &seen, match[1])
	}

	// 2. JSON-like keys
	for _, match := range stlJsonPattern.FindAllStringSubmatch(html, -1) {
		if len(match) < 2 {
			continue
		}
		addSTLLink(&files, &seen, match[1])
	}

	// 3. bare filenames (Printables style - "NeptuneGearHousing.stl")
	rawBare := regexp.MustCompile(`(?i)"?([A-Za-z0-9_.-]+\.stl)"?`)
	for _, match := range rawBare.FindAllStringSubmatch(html, -1) {
		if len(match) < 2 {
			continue
		}
		name := match[1]
		if !seen[name] {
			seen[name] = true
			files = append(files, ModelImportFile{Name: name, URL: name})
		}
	}

	return files
}

func addSTLLink(files *[]ModelImportFile, seen *map[string]bool, raw string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return
	}
	if (*seen)[raw] {
		return
	}
	(*seen)[raw] = true
	name := filepath.Base(strings.Split(raw, "?")[0])
	*files = append(*files, ModelImportFile{Name: name, URL: raw})
}

func resolveURL(base, ref string) string {
	if ref == "" {
		return ""
	}
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return ref
	}
	// bare filename → keep as-is
	if !strings.Contains(ref, "/") {
		return ref
	}
	u, err := url.Parse(base)
	if err != nil {
		return ref
	}
	refURL, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	res := u.ResolveReference(refURL).String()
	// if result ends with the original filename, use it
	if strings.HasSuffix(res, filepath.Base(ref)) {
		return res
	}
	return ref
}

func htmlUnescape(value string) string {
	return strings.NewReplacer("&amp;", "&", "&quot;", "\"", "&#39;", "'", "&lt;", "<", "&gt;", ">").Replace(value)
}

func (s *ModelImportService) enrichPrintablesFiles(ctx context.Context, pageURL string, files []ModelImportFile) []ModelImportFile {
	reID := regexp.MustCompile(`/model/(\d+)`)
	m := reID.FindStringSubmatch(pageURL)
	if len(m) != 2 {
		return files
	}
	modelID := m[1]

	query := `{"query":"query ModelFiles($id:ID!){model(id:$id){files{id,name,downloadUrl}}}","variables":{"id":"` + modelID + `"}}`

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.printables.com/graphql", strings.NewReader(query))
	if err != nil {
		return files
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Picofarm/1.0)")

	resp, err := s.client.Do(req)
	if err != nil {
		return files
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return files
	}

	var gql struct {
		Data struct {
			Model struct {
				Files []struct {
					ID          string `json:"id"`
					Name        string `json:"name"`
					DownloadURL string `json:"downloadUrl"`
				} `json:"files"`
			} `json:"model"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&gql); err != nil {
		return files
	}

	nameToURL := map[string]string{}
	for _, f := range gql.Data.Model.Files {
		if f.DownloadURL != "" {
			nameToURL[f.Name] = f.DownloadURL
		}
	}

	result := make([]ModelImportFile, 0, len(files))
	for _, f := range files {
		if dl, ok := nameToURL[f.Name]; ok {
			result = append(result, ModelImportFile{Name: f.Name, URL: dl})
		} else {
			result = append(result, f)
		}
	}
	return result
}
