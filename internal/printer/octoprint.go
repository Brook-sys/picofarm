package printer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/google/uuid"
)

// OctoPrintClient implements Client for OctoPrint API.
type OctoPrintClient struct {
	printerID      uuid.UUID
	baseURL        string
	apiKey         string
	httpClient     *http.Client
	statusCallback func(*model.PrinterState)
	stopPolling    chan struct{}
}

// NewOctoPrintClient creates a new OctoPrint client.
func NewOctoPrintClient(printerID uuid.UUID, baseURL string, apiKey string) *OctoPrintClient {
	return &OctoPrintClient{
		printerID: printerID,
		baseURL:   baseURL,
		apiKey:    apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		stopPolling: make(chan struct{}),
	}
}

// Connect establishes connection and starts status polling.
func (c *OctoPrintClient) Connect() error {
	// Verify connection by getting version
	_, err := c.doRequest("GET", "/api/version", nil)
	if err != nil {
		return fmt.Errorf("failed to connect to OctoPrint: %w", err)
	}

	// Start polling for status updates
	go c.pollStatus()

	return nil
}

// Disconnect stops polling and closes connection.
func (c *OctoPrintClient) Disconnect() error {
	close(c.stopPolling)
	return nil
}

// GetStatus retrieves current printer status.
func (c *OctoPrintClient) GetStatus() (*model.PrinterState, error) {
	// Get printer state
	printerResp, err := c.doRequest("GET", "/api/printer", nil)
	if err != nil {
		// Connection failure means the printer is offline, not an application error
		offlineState := &model.PrinterState{PrinterID: c.printerID, Status: model.PrinterStatusOffline, UpdatedAt: time.Now()}
		return offlineState, nil //nolint:nilerr
	}

	// Get job state
	jobResp, err := c.doRequest("GET", "/api/job", nil)
	if err != nil {
		jobResp = []byte("{}")
	}

	state := c.parseState(printerResp, jobResp)
	return state, nil
}

// StartJob uploads and starts printing a file.
func (c *OctoPrintClient) ListFiles(ctx context.Context, dir string) ([]model.PrinterFileEntry, error) {
	_ = ctx
	endpoint := "/api/files/local"
	if strings.TrimSpace(dir) != "" {
		// OctoPrint directory lookup (recursive tree, so we need to navigate or request specific folder)
		// OctoPrint doesn't have a direct "list single folder" API easily, but it allows fetching files.
		// For simplicity, fetch all and filter or fetch folder by name.
		// Note: /api/files/local?recursive=true
	}
	resp, err := c.doRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Files []octoprintFileEntry `json:"files"`
	}
	if err := json.Unmarshal(resp, &payload); err != nil {
		return nil, err
	}

	// Helper to extract
	var entries []model.PrinterFileEntry
	for _, item := range payload.Files {
		// We only process top-level for now unless recursive parsed
		entries = append(entries, item.toModel(""))
	}
	return entries, nil
}

func (c *OctoPrintClient) UploadFile(ctx context.Context, dir string, filename string, file io.Reader) error {
	_ = ctx
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	if dir != "" {
		if err := writer.WriteField("path", dir); err != nil {
			return err
		}
	}

	part, err := writer.CreateFormFile("file", filepath.Base(filename))
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, file); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	req, err := http.NewRequest("POST", c.baseURL+"/api/files/local", body)
	if err != nil {
		return err
	}
	req.Header.Set("X-Api-Key", c.apiKey)
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

func (c *OctoPrintClient) DeleteFile(ctx context.Context, filePath string) error {
	_ = ctx
	_, err := c.doRequest("DELETE", "/api/files/local/"+escapeMoonrakerPath(filePath), nil)
	return err
}

func (c *OctoPrintClient) CreateDirectory(ctx context.Context, dirPath string) error {
	_ = ctx
	// OctoPrint creates folders via POST /api/files/local
	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)
	writer.WriteField("foldername", filepath.Base(dirPath))
	writer.WriteField("path", filepath.Dir(dirPath))
	writer.Close()

	req, err := http.NewRequest("POST", c.baseURL+"/api/files/local", payload)
	if err != nil {
		return err
	}
	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("mkdir failed: %d", resp.StatusCode)
	}
	return nil
}

func (c *OctoPrintClient) RenameFile(ctx context.Context, oldPath string, newPath string) error {
	return fmt.Errorf("not supported on OctoPrint")
}

func (c *OctoPrintClient) MoveFile(ctx context.Context, sourcePath string, destPath string) error {
	return fmt.Errorf("not supported on OctoPrint")
}

func (c *OctoPrintClient) DownloadFile(ctx context.Context, filePath string) (io.ReadCloser, error) {
	_ = ctx
	req, err := http.NewRequest("GET", c.baseURL+"/downloads/files/local/"+escapeMoonrakerPath(filePath), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Api-Key", c.apiKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		return nil, fmt.Errorf("download failed: %d", resp.StatusCode)
	}
	return resp.Body, nil
}

func (c *OctoPrintClient) GetFileMetadata(ctx context.Context, filePath string) (*model.PrinterFileMetadata, error) {
	_ = ctx
	// OctoPrint metadata
	resp, err := c.doRequest("GET", "/api/files/local/"+escapeMoonrakerPath(filePath), nil)
	if err != nil {
		return nil, err
	}
	var payload octoprintFileEntry
	if err := json.Unmarshal(resp, &payload); err != nil {
		return nil, err
	}
	return &model.PrinterFileMetadata{
		Path:          filePath,
		Size:          payload.Size,
		Modified:      int64(payload.Date),
		EstimatedTime: float64(payload.GcodeAnalysis.EstimatedPrintTime),
		FilamentTotal: payload.GcodeAnalysis.Filament.Tool0.Length,
	}, nil
}

func (c *OctoPrintClient) DownloadThumbnail(ctx context.Context, thumbPath string) (io.ReadCloser, error) {
	return nil, fmt.Errorf("not supported on OctoPrint")
}

type octoprintFileEntry struct {
	Name          string   `json:"name"`
	Path          string   `json:"path"`
	Type          string   `json:"type"` // machinecode, folder
	TypePath      []string `json:"typePath"`
	Size          int64    `json:"size"`
	Date          int64    `json:"date"`
	GcodeAnalysis struct {
		EstimatedPrintTime int `json:"estimatedPrintTime"`
		Filament           struct {
			Tool0 struct {
				Length float64 `json:"length"`
			} `json:"tool0"`
		} `json:"filament"`
	} `json:"gcodeAnalysis"`
}

func (e octoprintFileEntry) toModel(parent string) model.PrinterFileEntry {
	entryType := "file"
	if e.Type == "folder" {
		entryType = "dir"
	}
	return model.PrinterFileEntry{
		Path:     e.Path,
		Name:     e.Name,
		Type:     entryType,
		Size:     e.Size,
		Modified: e.Date,
	}
}

func (c *OctoPrintClient) StartPrint(ctx context.Context, filePath string) error {
	_ = ctx
	selectReq := map[string]interface{}{
		"command": "select",
		"print":   true,
	}
	body, _ := json.Marshal(selectReq)
	_, err := c.doRequest("POST", "/api/files/local/"+escapeMoonrakerPath(filePath), body)
	return err
}
func (c *OctoPrintClient) StartJob(filename string, filepath string) error {
	// Upload file
	fileReader, err := os.Open(filepath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer fileReader.Close()
	if err := c.UploadFile(context.Background(), "", filename, fileReader); err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}

	// Start print
	selectReq := map[string]interface{}{
		"command": "select",
		"print":   true,
	}
	body, _ := json.Marshal(selectReq)

	_, err = c.doRequest("POST", "/api/files/local/"+filename, body)
	if err != nil {
		return fmt.Errorf("failed to start print: %w", err)
	}

	return nil
}

// PauseJob pauses the current print.
func (c *OctoPrintClient) PauseJob() error {
	req := map[string]string{"command": "pause", "action": "pause"}
	body, _ := json.Marshal(req)
	_, err := c.doRequest("POST", "/api/job", body)
	return err
}

// ResumeJob resumes a paused print.
func (c *OctoPrintClient) ResumeJob() error {
	req := map[string]string{"command": "pause", "action": "resume"}
	body, _ := json.Marshal(req)
	_, err := c.doRequest("POST", "/api/job", body)
	return err
}

// CancelJob cancels the current print.
func (c *OctoPrintClient) CancelJob() error {
	req := map[string]string{"command": "cancel"}
	body, _ := json.Marshal(req)
	_, err := c.doRequest("POST", "/api/job", body)
	return err
}

func (c *OctoPrintClient) Capabilities() model.PrinterCapabilities {
	return model.PrinterCapabilities{
		CanStartPrint:   true,
		CanPause:        true,
		CanResume:       true,
		CanCancel:       true,
		CanRunGCode:     true,
		CanSetFeedRate:  true,
		CanUploadFile:   true,
		CanControlTemps: true,
	}
}

func (c *OctoPrintClient) SetFeedRate(percent int) error {
	req := map[string][]string{"commands": []string{fmt.Sprintf("M220 S%d", percent)}}
	body, _ := json.Marshal(req)
	_, err := c.doRequest("POST", "/api/printer/command", body)
	return err
}

func (c *OctoPrintClient) RunMacro(name string) error {
	macro := strings.TrimSpace(name)
	if macro == "" {
		return fmt.Errorf("macro name is required")
	}
	req := map[string][]string{"commands": []string{macro}}
	body, _ := json.Marshal(req)
	_, err := c.doRequest("POST", "/api/printer/command", body)
	return err
}

// SetStatusCallback sets the callback for status updates.
func (c *OctoPrintClient) SetStatusCallback(cb func(*model.PrinterState)) {
	c.statusCallback = cb
}

// doRequest performs an HTTP request to the OctoPrint API.
func (c *OctoPrintClient) doRequest(method string, path string, body []byte) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Api-Key", c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// pollStatus periodically polls for status updates.
func (c *OctoPrintClient) pollStatus() {
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

// parseState converts OctoPrint API responses to PrinterState.
func (c *OctoPrintClient) parseState(printerResp []byte, jobResp []byte) *model.PrinterState {
	state := &model.PrinterState{
		PrinterID: c.printerID,
		Status:    model.PrinterStatusIdle,
		UpdatedAt: time.Now(),
	}

	// Parse printer response
	var printerData struct {
		State struct {
			Text  string `json:"text"`
			Flags struct {
				Printing bool `json:"printing"`
				Paused   bool `json:"paused"`
				Error    bool `json:"error"`
				Ready    bool `json:"ready"`
			} `json:"flags"`
		} `json:"state"`
		Temperature struct {
			Bed struct {
				Actual float64 `json:"actual"`
			} `json:"bed"`
			Tool0 struct {
				Actual float64 `json:"actual"`
			} `json:"tool0"`
		} `json:"temperature"`
	}

	if err := json.Unmarshal(printerResp, &printerData); err == nil {
		state.BedTemp = printerData.Temperature.Bed.Actual
		state.NozzleTemp = printerData.Temperature.Tool0.Actual

		if printerData.State.Flags.Error {
			state.Status = model.PrinterStatusError
		} else if printerData.State.Flags.Paused {
			state.Status = model.PrinterStatusPaused
		} else if printerData.State.Flags.Printing {
			state.Status = model.PrinterStatusPrinting
		} else if printerData.State.Flags.Ready {
			state.Status = model.PrinterStatusIdle
		}
	}

	// Parse job response
	var jobData struct {
		Job struct {
			File struct {
				Name string `json:"name"`
			} `json:"file"`
		} `json:"job"`
		Progress struct {
			Completion    float64 `json:"completion"`
			PrintTimeLeft int     `json:"printTimeLeft"`
		} `json:"progress"`
	}

	if err := json.Unmarshal(jobResp, &jobData); err == nil {
		state.CurrentFile = jobData.Job.File.Name
		state.Progress = jobData.Progress.Completion
		state.TimeLeft = jobData.Progress.PrintTimeLeft
	}

	return state
}
