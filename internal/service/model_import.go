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
	files    *FileService
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
	PDFURL      string            `json:"pdf_url,omitempty"`
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

func NewModelImportService(projects *ProjectService, parts *PartService, designs *DesignService, stls *STLLibraryService, files *FileService, tags *repository.TagRepository) *ModelImportService {
	return &ModelImportService{projects: projects, parts: parts, designs: designs, stls: stls, files: files, tags: tags, client: &http.Client{Timeout: 45 * time.Second}}
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

	// Enrich Printables metadata from the embedded SvelteKit payload.
	if provider == "Printables" {
		s.enrichPrintablesEmbedded(body, preview)
		if enriched := s.enrichPrintablesFiles(ctx, preview, preview.STLFiles); len(enriched) > 0 {
			preview.STLFiles = enriched
		}
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
	if preview.ImageURL != "" && s.files != nil {
		if cover, err := s.downloadImage(ctx, preview.ImageURL); err == nil {
			project.CoverFileID = &cover.ID
		}
	}
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

func (s *ModelImportService) downloadImage(ctx context.Context, imageURL string) (*model.File, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
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
		return nil, fmt.Errorf("failed to download image: %d", resp.StatusCode)
	}
	name := filepath.Base(strings.Split(imageURL, "?")[0])
	if name == "" || name == "." || name == "/" {
		name = "cover.jpg"
	}
	return s.files.Upload(ctx, name, resp.Body)
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

func (s *ModelImportService) enrichPrintablesEmbedded(body string, preview *ModelImportPreview) {
	for _, match := range regexp.MustCompile(`(?is)<script[^>]+data-sveltekit-fetched[^>]*>(.*?)</script>`).FindAllStringSubmatch(body, -1) {
		if len(match) < 2 {
			continue
		}
		var outer struct {
			Body string `json:"body"`
		}
		if err := json.Unmarshal([]byte(htmlUnescape(match[1])), &outer); err != nil || outer.Body == "" {
			continue
		}
		var payload struct {
			Data struct {
				Model struct {
					ID          string `json:"id"`
					Name        string `json:"name"`
					Description string `json:"description"`
					Summary     string `json:"summary"`
					PDFFilePath string `json:"pdfFilePath"`
					Image       struct {
						FilePath string `json:"filePath"`
					} `json:"image"`
					User struct {
						PublicUsername string `json:"publicUsername"`
					} `json:"user"`
					License struct {
						ID                string `json:"id"`
						DisallowRemixing  bool   `json:"disallowRemixing"`
						DisallowCommercial bool   `json:"disallowCommercial"`
					} `json:"license"`
					FilesCount int `json:"filesCount"`
				} `json:"model"`
			} `json:"data"`
		}
		if err := json.Unmarshal([]byte(outer.Body), &payload); err != nil || payload.Data.Model.ID == "" {
			continue
		}
		model := payload.Data.Model
		if model.Name != "" {
			preview.Title = model.Name
		}
		if model.Description != "" {
			preview.Description = cleanHTMLText(model.Description)
		} else if model.Summary != "" {
			preview.Description = htmlUnescape(model.Summary)
		}
		if model.User.PublicUsername != "" {
			preview.Author = model.User.PublicUsername
		}
		if model.License.ID != "" {
			preview.License = printablesLicenseName(model.License.ID, model.License.DisallowRemixing, model.License.DisallowCommercial)
		}
		if model.Image.FilePath != "" {
			preview.ImageURL = printablesMediaURL(model.Image.FilePath)
		}
		if model.PDFFilePath != "" {
			preview.PDFURL = printablesMediaURL(model.PDFFilePath)
		}
		return
	}
}

func printablesMediaURL(filePath string) string {
	if filePath == "" || strings.HasPrefix(filePath, "http") {
		return filePath
	}
	return "https://media.printables.com/" + strings.TrimLeft(filePath, "/")
}

func printablesLicenseName(id string, disallowRemixing, disallowCommercial bool) string {
	switch id {
	case "1":
		return "Public Domain"
	case "2":
		return "Creative Commons Attribution"
	case "3":
		return "Creative Commons Attribution-ShareAlike"
	case "4":
		return "Creative Commons Attribution-NonCommercial"
	case "5":
		return "Creative Commons Attribution-NonCommercial-ShareAlike"
	case "6":
		return "Creative Commons Attribution-NoDerivatives"
	case "7":
		return "Creative Commons Attribution-NonCommercial-NoDerivatives"
	}
	parts := []string{"Printables license"}
	if disallowCommercial {
		parts = append(parts, "non-commercial")
	}
	if disallowRemixing {
		parts = append(parts, "no derivatives")
	}
	return strings.Join(parts, ", ")
}

func cleanHTMLText(value string) string {
	value = regexp.MustCompile(`(?i)<br\s*/?>`).ReplaceAllString(value, "\n")
	value = regexp.MustCompile(`(?i)</p>|</li>|</h[1-6]>`).ReplaceAllString(value, "\n")
	value = regexp.MustCompile(`(?i)<li[^>]*>`).ReplaceAllString(value, "- ")
	value = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(value, "")
	value = htmlUnescape(value)
	lines := strings.Split(value, "\n")
	clean := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			clean = append(clean, line)
		}
	}
	return strings.Join(clean, "\n")
}

func (s *ModelImportService) enrichPrintablesFiles(ctx context.Context, preview *ModelImportPreview, files []ModelImportFile) []ModelImportFile {
	reID := regexp.MustCompile(`/model/(\d+)`)
	m := reID.FindStringSubmatch(preview.SourceURL)
	if len(m) != 2 {
		return files
	}
	modelID := m[1]

	query := `{"query":"query ModelDetails($id:ID!){model(id:$id){name,description,user{publicUsername},license{name},files{id,name,downloadUrl}}}","variables":{"id":"` + modelID + `"}}`

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
				Name        string `json:"name"`
				Description string `json:"description"`
				User        struct {
					PublicUsername string `json:"publicUsername"`
				} `json:"user"`
				License struct {
					Name string `json:"name"`
				} `json:"license"`
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

	// Update preview with GraphQL details which are usually better/cleaner than meta tags
	if gql.Data.Model.Name != "" {
		preview.Title = gql.Data.Model.Name
	}
	if gql.Data.Model.User.PublicUsername != "" {
		preview.Author = gql.Data.Model.User.PublicUsername
	}
	if gql.Data.Model.License.Name != "" {
		preview.License = gql.Data.Model.License.Name
	}
	if gql.Data.Model.Description != "" {
		preview.Description = cleanHTMLText(gql.Data.Model.Description)
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
