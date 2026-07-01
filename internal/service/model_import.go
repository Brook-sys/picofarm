package service

import (
	"context"
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
	stlLinkPattern = regexp.MustCompile(`(?i)https?://[^"'<>\\s]+\.stl(?:\?[^"'<>\\s]*)?`)
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
	preview := &ModelImportPreview{
		Provider:    provider,
		SourceURL:   rawURL,
		Title:       htmlUnescape(strings.TrimSpace(title)),
		Description: htmlUnescape(firstNonEmpty(meta["og:description"], meta["description"], meta["twitter:description"])),
		Author:      htmlUnescape(firstNonEmpty(meta["author"], meta["article:author"])),
		License:     htmlUnescape(firstNonEmpty(meta["license"])),
		ImageURL:    firstNonEmpty(meta["og:image"], meta["twitter:image"]),
		STLFiles:    extractSTLLinks(body),
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, file.URL, nil)
	if err != nil {
		return nil, err
	}
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
		name = filepath.Base(strings.Split(file.URL, "?")[0])
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
	for _, raw := range stlLinkPattern.FindAllString(html, -1) {
		if seen[raw] {
			continue
		}
		seen[raw] = true
		name := filepath.Base(strings.Split(raw, "?")[0])
		files = append(files, ModelImportFile{Name: name, URL: raw})
	}
	return files
}

func htmlUnescape(value string) string {
	return strings.NewReplacer("&amp;", "&", "&quot;", "\"", "&#39;", "'", "&lt;", "<", "&gt;", ">").Replace(value)
}
