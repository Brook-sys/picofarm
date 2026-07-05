package printer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/google/uuid"
)

// MoonrakerClient implements Client for Moonraker API (Klipper).
type MoonrakerClient struct {
	printerID      uuid.UUID
	baseURL        string
	httpClient     *http.Client
	statusCallback func(*model.PrinterState)
	stopPolling    chan struct{}
}

// NewMoonrakerClient creates a new Moonraker client.
func NewMoonrakerClient(printerID uuid.UUID, baseURL string) *MoonrakerClient {
	// Ensure no trailing slash to avoid double slashes in requests
	baseURL = strings.TrimRight(baseURL, "/")
	// If no port specified, default to Moonraker port 7125
	if u, err := url.Parse(baseURL); err == nil && u.Host != "" {
		if _, _, perr := net.SplitHostPort(u.Host); perr != nil {
			if u.Port() == "" {
				u.Host = net.JoinHostPort(u.Hostname(), "7125")
				baseURL = u.String()
			}
		}
	}
	return &MoonrakerClient{
		printerID: printerID,
		baseURL:   baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		stopPolling: make(chan struct{}),
	}
}

// Connect establishes connection and starts status polling.
func (c *MoonrakerClient) Connect() error {
	// Verify connection by getting server info
	_, err := c.doRequest("GET", "/server/info", nil)
	if err != nil {
		return fmt.Errorf("failed to connect to Moonraker: %w", err)
	}

	go c.pollStatus()
	return nil
}

// Disconnect stops polling.
func (c *MoonrakerClient) Disconnect() error {
	close(c.stopPolling)
	return nil
}

// GetStatus retrieves current printer status.
func (c *MoonrakerClient) GetStatus() (*model.PrinterState, error) {
	// Get printer status
	resp, err := c.doRequest("GET", "/printer/objects/query?print_stats&display_status&extruder&heater_bed", nil)
	if err != nil {
		// Connection failure means the printer is offline, not an application error
		offlineState := &model.PrinterState{PrinterID: c.printerID, Status: model.PrinterStatusOffline, UpdatedAt: time.Now()}
		return offlineState, nil //nolint:nilerr
	}

	state := c.parseState(resp)
	return state, nil
}

// StartJob uploads and starts printing a file.
func (c *MoonrakerClient) StartJob(filename string, filePath string) error {
	remoteName, err := c.uploadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}
	if remoteName == "" {
		remoteName = filename
	}

	_, err = c.doRequest("POST", "/printer/print/start?filename="+url.QueryEscape(remoteName), nil)
	if err != nil {
		return fmt.Errorf("failed to start print: %w", err)
	}

	return nil
}

// PauseJob pauses the current print.
func (c *MoonrakerClient) PauseJob() error {
	_, err := c.doRequest("POST", "/printer/print/pause", nil)
	return err
}

// ResumeJob resumes a paused print.
func (c *MoonrakerClient) ResumeJob() error {
	_, err := c.doRequest("POST", "/printer/print/resume", nil)
	return err
}

// CancelJob cancels the current print.
func (c *MoonrakerClient) CancelJob() error {
	_, err := c.doRequest("POST", "/printer/print/cancel", nil)
	return err
}

func (c *MoonrakerClient) Capabilities() model.PrinterCapabilities {
	return model.PrinterCapabilities{
		CanStartPrint:        true,
		CanPause:             true,
		CanResume:            true,
		CanCancel:            true,
		CanRunGCode:          true,
		CanSetFeedRate:       true,
		CanUploadFile:        true,
		CanControlTemps:      true,
		CanConfirmPlateClear: true,
	}
}

func (c *MoonrakerClient) SetFeedRate(percent int) error {
	return c.RunMacro(fmt.Sprintf("M220 S%d", percent))
}

// RunMacro executes a Klipper macro or raw G-code script through Moonraker.
func (c *MoonrakerClient) RunMacro(name string) error {
	macro := strings.TrimSpace(name)
	if macro == "" {
		return fmt.Errorf("macro name is required")
	}
	_, err := c.doRequest("POST", "/printer/gcode/script?script="+url.QueryEscape(macro), nil)
	return err
}

// SetStatusCallback sets the callback for status updates.
func (c *MoonrakerClient) SetStatusCallback(cb func(*model.PrinterState)) {
	c.statusCallback = cb
}

// doRequest performs an HTTP request to the Moonraker API.
func (c *MoonrakerClient) doRequest(method string, path string, body []byte) ([]byte, error) { //nolint:unparam // body kept for future POST/PUT support
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %d %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return io.ReadAll(resp.Body)
}

// uploadFile uploads a file to Moonraker. Returns remote filename on success.
func (c *MoonrakerClient) uploadFile(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return "", err
	}

	if _, err := io.Copy(part, file); err != nil {
		return "", err
	}

	writer.Close()

	req, err := http.NewRequest("POST", c.baseURL+"/server/files/upload", body)
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("upload failed: %d %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	// Moonraker returns JSON {result: {item: {path, ...}}}
	var uploadResp struct {
		Result struct {
			Item struct {
				Path string `json:"path"`
			} `json:"item"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&uploadResp); err == nil {
		if uploadResp.Result.Item.Path != "" {
			return uploadResp.Result.Item.Path, nil
		}
	}
	return filepath.Base(filePath), nil
}

// pollStatus periodically polls for status updates.
func (c *MoonrakerClient) pollStatus() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopPolling:
			return
		case <-ticker.C:
			state, err := c.GetStatus()
			if err == nil && c.statusCallback != nil {
				c.statusCallback(state)
			}
		}
	}
}

// parseState converts Moonraker API response to PrinterState.
func (c *MoonrakerClient) parseState(resp []byte) *model.PrinterState {
	state := &model.PrinterState{
		PrinterID: c.printerID,
		Status:    model.PrinterStatusIdle,
		UpdatedAt: time.Now(),
	}

	var data struct {
		Result struct {
			Status struct {
				PrintStats struct {
					State         string  `json:"state"`
					Filename      string  `json:"filename"`
					TotalDuration float64 `json:"total_duration"`
					PrintDuration float64 `json:"print_duration"`
				} `json:"print_stats"`
				Extruder struct {
					Temperature float64 `json:"temperature"`
				} `json:"extruder"`
				HeaterBed struct {
					Temperature float64 `json:"temperature"`
				} `json:"heater_bed"`
				DisplayStatus struct {
					Progress float64 `json:"progress"`
				} `json:"display_status"`
			} `json:"status"`
		} `json:"result"`
	}

	if err := json.Unmarshal(resp, &data); err == nil {
		state.NozzleTemp = data.Result.Status.Extruder.Temperature
		state.BedTemp = data.Result.Status.HeaterBed.Temperature
		state.Progress = data.Result.Status.DisplayStatus.Progress * 100
		state.CurrentFile = data.Result.Status.PrintStats.Filename

		switch data.Result.Status.PrintStats.State {
		case "printing":
			state.Status = model.PrinterStatusPrinting
		case "paused":
			state.Status = model.PrinterStatusPaused
		case "error":
			state.Status = model.PrinterStatusError
		case "standby", "complete":
			state.Status = model.PrinterStatusIdle
		}
	}

	return state
}

func (c *MoonrakerClient) ListFiles(ctx context.Context, dir string) ([]model.PrinterFileEntry, error) {
	_ = ctx
	cleanDir := normalizeMoonrakerGCodeRelativePath(dir)
	endpoints := []string{
		"/server/files/directory?root=gcodes",
		"/server/files/directory?path=" + url.QueryEscape("gcodes"),
	}
	if cleanDir != "" {
		endpoints = []string{
			"/server/files/directory?path=" + url.QueryEscape("gcodes/"+cleanDir),
			"/server/files/directory?root=gcodes&path=" + url.QueryEscape(cleanDir),
		}
		if cleanDir == "sda1" || strings.HasPrefix(cleanDir, "sda1/") {
			sdaPath := strings.TrimPrefix(cleanDir, "sda1")
			sdaPath = strings.TrimPrefix(sdaPath, "/")
			if sdaPath == "" {
				endpoints = append(endpoints,
					"/server/files/directory?root=sda1",
					"/server/files/directory?path="+url.QueryEscape("sda1"),
				)
			} else {
				endpoints = append(endpoints,
					"/server/files/directory?root=sda1&path="+url.QueryEscape(sdaPath),
					"/server/files/directory?path="+url.QueryEscape("sda1/"+sdaPath),
				)
			}
		}
	}
	var firstEntries []model.PrinterFileEntry
	var firstErr error
	for _, endpoint := range endpoints {
		entries, err := c.listFilesEndpoint(endpoint, cleanDir)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if firstEntries == nil {
			firstEntries = entries
		}
		if len(entries) > 0 {
			return entries, nil
		}
	}
	fallbackEntries, fallbackErr := c.listFilesRecursiveFallback(cleanDir, "gcodes")
	if fallbackErr == nil && len(fallbackEntries) > 0 {
		return fallbackEntries, nil
	}
	if cleanDir == "sda1" || strings.HasPrefix(cleanDir, "sda1/") {
		sdaEntries, sdaErr := c.listFilesRecursiveFallback(cleanDir, "sda1")
		if sdaErr == nil && len(sdaEntries) > 0 {
			return sdaEntries, nil
		}
	}
	if firstEntries != nil {
		return firstEntries, nil
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return nil, fallbackErr
}

func (c *MoonrakerClient) listFilesEndpoint(endpoint string, baseDir string) ([]model.PrinterFileEntry, error) {
	resp, err := c.doRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Result struct {
			Dirs  []moonrakerFileEntry `json:"dirs"`
			Files []moonrakerFileEntry `json:"files"`
		} `json:"result"`
	}
	if err := json.Unmarshal(resp, &payload); err != nil {
		return nil, err
	}
	entries := make([]model.PrinterFileEntry, 0, len(payload.Result.Dirs)+len(payload.Result.Files))
	for _, item := range payload.Result.Dirs {
		entries = append(entries, item.toModel("dir", baseDir))
	}
	for _, item := range payload.Result.Files {
		entries = append(entries, item.toModel("file", baseDir))
	}
	return entries, nil
}

func (c *MoonrakerClient) listFilesRecursiveFallback(baseDir string, root string) ([]model.PrinterFileEntry, error) {
	resp, err := c.doRequest("GET", "/server/files/list?root="+url.QueryEscape(root), nil)
	if err != nil {
		return nil, err
	}
	var raw struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(resp, &raw); err != nil {
		return nil, err
	}
	files := []moonrakerFileEntry{}
	if err := json.Unmarshal(raw.Result, &files); err != nil {
		var wrapped struct {
			Files []moonrakerFileEntry `json:"files"`
		}
		if err := json.Unmarshal(raw.Result, &wrapped); err != nil {
			return nil, err
		}
		files = wrapped.Files
	}
	entries := []model.PrinterFileEntry{}
	seenDirs := map[string]bool{}
	for _, item := range files {
		filePath := normalizeMoonrakerGCodeRelativePath(item.Path)
		if filePath == "" {
			filePath = normalizeMoonrakerGCodeRelativePath(item.Filename)
		}
		if root == "sda1" && filePath != "" && filePath != "sda1" && !strings.HasPrefix(filePath, "sda1/") {
			filePath = path.Join("sda1", filePath)
		}
		if filePath == "" || !isDirectMoonrakerChild(filePath, baseDir) {
			continue
		}
		relative := strings.TrimPrefix(filePath, strings.TrimSuffix(baseDir, "/")+"/")
		if baseDir == "" {
			relative = filePath
		}
		parts := strings.Split(relative, "/")
		if len(parts) > 1 {
			dirPath := path.Join(baseDir, parts[0])
			if !seenDirs[dirPath] {
				seenDirs[dirPath] = true
				entries = append(entries, model.PrinterFileEntry{Path: dirPath, Name: parts[0], Type: "dir"})
			}
			continue
		}
		item.Path = filePath
		entries = append(entries, item.toModel("file", baseDir))
	}
	return entries, nil
}

func isDirectMoonrakerChild(filePath string, baseDir string) bool {
	if baseDir == "" {
		return filePath != ""
	}
	return filePath != baseDir && strings.HasPrefix(filePath, strings.TrimSuffix(baseDir, "/")+"/")
}

func (c *MoonrakerClient) UploadFile(ctx context.Context, dir string, filename string, file io.Reader) error {
	_ = ctx
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filepath.Base(filename))
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, file); err != nil {
		return err
	}
	if strings.TrimSpace(dir) != "" {
		if err := writer.WriteField("path", dir); err != nil {
			return err
		}
	}
	if err := writer.WriteField("root", "gcodes"); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	req, err := http.NewRequest("POST", c.baseURL+"/server/files/upload", body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed: %d %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}
	return nil
}

func (c *MoonrakerClient) DeleteFile(ctx context.Context, filePath string) error {
	_ = ctx
	_, err := c.doRequest("DELETE", "/server/files/gcodes/"+escapeMoonrakerPath(filePath), nil)
	return err
}

func (c *MoonrakerClient) DeleteDirectory(ctx context.Context, dirPath string) error {
	_ = ctx
	_, err := c.doRequest("DELETE", "/server/files/directory?path="+url.QueryEscape("gcodes/"+strings.TrimPrefix(dirPath, "/")), nil)
	return err
}

func (c *MoonrakerClient) CreateDirectory(ctx context.Context, dirPath string) error {
	_ = ctx
	_, err := c.doRequest("POST", "/server/files/directory?root=gcodes&path="+url.QueryEscape(strings.TrimPrefix(dirPath, "/")), nil)
	return err
}

func (c *MoonrakerClient) RenameFile(ctx context.Context, oldPath string, newPath string) error {
	_ = ctx
	sourcePath := moonrakerGCodePath(oldPath)
	destPath := moonrakerGCodePath(newPath)
	payload, err := json.Marshal(map[string]string{
		"source": sourcePath,
		"dest":   destPath,
	})
	if err != nil {
		return err
	}
	if _, err = c.doRequest("POST", "/server/files/move", payload); err != nil {
		return err
	}
	verifyPath := strings.TrimPrefix(destPath, "gcodes/")
	if path.Ext(verifyPath) != "" {
		if _, err := c.GetFileMetadata(ctx, verifyPath); err != nil {
			return fmt.Errorf("file was moved but destination could not be verified: %w", err)
		}
	}
	return nil
}

func (c *MoonrakerClient) MoveFile(ctx context.Context, sourcePath string, destPath string) error {
	return c.RenameFile(ctx, sourcePath, destPath)
}

func (c *MoonrakerClient) DownloadFile(ctx context.Context, filePath string) (io.ReadCloser, error) {
	_ = ctx
	req, err := http.NewRequest("GET", c.baseURL+"/server/files/gcodes/"+escapeMoonrakerPath(filePath), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("download failed: %d %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}
	return resp.Body, nil
}

func (c *MoonrakerClient) GetFileMetadata(ctx context.Context, filePath string) (*model.PrinterFileMetadata, error) {
	_ = ctx
	resp, err := c.doRequest("GET", "/server/files/metadata?filename="+url.QueryEscape(strings.TrimPrefix(filePath, "/")), nil)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Result struct {
			Size               int64   `json:"size"`
			Modified           float64 `json:"modified"`
			EstimatedTime      float64 `json:"estimated_time"`
			FilamentTotal      float64 `json:"filament_total"`
			LayerHeight        float64 `json:"layer_height"`
			ObjectHeight       float64 `json:"object_height"`
			FirstLayerHeight   float64 `json:"first_layer_height"`
			FirstLayerBedTemp  float64 `json:"first_layer_bed_temp"`
			FirstLayerExtrTemp float64 `json:"first_layer_extr_temp"`
			Slicer             string  `json:"slicer"`
			SlicerVersion      string  `json:"slicer_version"`
			Thumbnails         []struct {
				RelativePath string `json:"relative_path"`
			} `json:"thumbnails"`
		} `json:"result"`
	}
	if err := json.Unmarshal(resp, &payload); err != nil {
		return nil, err
	}
	metadata := &model.PrinterFileMetadata{Path: strings.TrimPrefix(filePath, "/"), Size: payload.Result.Size, Modified: int64(payload.Result.Modified), EstimatedTime: payload.Result.EstimatedTime, FilamentTotal: payload.Result.FilamentTotal, LayerHeight: payload.Result.LayerHeight, ObjectHeight: payload.Result.ObjectHeight, FirstLayerHeight: payload.Result.FirstLayerHeight, FirstLayerBedTemp: payload.Result.FirstLayerBedTemp, FirstLayerExtrTemp: payload.Result.FirstLayerExtrTemp, Slicer: payload.Result.Slicer, SlicerVersion: payload.Result.SlicerVersion}
	for _, thumb := range payload.Result.Thumbnails {
		if thumb.RelativePath == "" {
			continue
		}
		metadata.Thumbnails = append(metadata.Thumbnails, thumb.RelativePath)
	}
	if len(metadata.Thumbnails) > 0 {
		metadata.ThumbnailRelative = metadata.Thumbnails[len(metadata.Thumbnails)-1]
	}
	return metadata, nil
}

func (c *MoonrakerClient) DownloadThumbnail(ctx context.Context, thumbPath string) (io.ReadCloser, error) {
	_ = ctx
	req, err := http.NewRequest("GET", c.baseURL+"/server/files/gcodes/"+escapeMoonrakerPath(thumbPath), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("thumbnail failed: %d %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}
	return resp.Body, nil
}

func (c *MoonrakerClient) StartPrint(ctx context.Context, filePath string) error {
	_ = ctx
	_, err := c.doRequest("POST", "/printer/print/start?filename="+url.QueryEscape(strings.TrimPrefix(filePath, "/")), nil)
	return err
}

func moonrakerGCodePath(filePath string) string {
	cleanPath := normalizeMoonrakerGCodeRelativePath(filePath)
	return "gcodes/" + cleanPath
}

func normalizeMoonrakerGCodeRelativePath(filePath string) string {
	cleanPath := strings.TrimPrefix(strings.TrimSpace(filePath), "/")
	cleanPath = strings.TrimPrefix(cleanPath, "gcodes/")
	if cleanPath == "gcodes" {
		return ""
	}
	return cleanPath
}

func escapeMoonrakerPath(filePath string) string {
	parts := strings.Split(strings.TrimPrefix(filePath, "/"), "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

type moonrakerFileEntry struct {
	Path     string  `json:"path"`
	Filename string  `json:"filename"`
	Dirname  string  `json:"dirname"`
	Root     string  `json:"root"`
	Size     int64   `json:"size"`
	Modified float64 `json:"modified"`
}

func (e moonrakerFileEntry) toModel(entryType string, baseDir string) model.PrinterFileEntry {
	name := e.Filename
	if entryType == "dir" {
		name = e.Dirname
	}
	if name == "" {
		name = path.Base(e.Path)
	}
	if name == "" || name == "." {
		name = "gcodes"
	}
	fullPath := normalizeMoonrakerGCodeRelativePath(e.Path)
	if fullPath == "" {
		fullPath = normalizeMoonrakerGCodeRelativePath(name)
	}
	if fullPath != "" && baseDir != "" && !strings.HasPrefix(fullPath, baseDir+"/") && fullPath != baseDir {
		fullPath = path.Join(baseDir, fullPath)
	}
	return model.PrinterFileEntry{
		Path:      fullPath,
		Name:      name,
		Type:      entryType,
		Size:      e.Size,
		Modified:  int64(e.Modified),
		Root:      e.Root,
		Extension: strings.TrimPrefix(strings.ToLower(path.Ext(fullPath)), "."),
	}
}
