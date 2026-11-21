package service

import (
	"encoding/json"
	"fmt"
	"math"
	"mime/multipart"
	"strconv"
)

// Keep these wire types aligned with slicer-api/internal/slicer/toolpath.go.
type SlicerToolpathPreview struct {
	Version          int                   `json:"version"`
	PositionUnit     string                `json:"positionUnit"`
	ExtrusionUnit    string                `json:"extrusionUnit"`
	FeedrateUnit     string                `json:"feedrateUnit"`
	CoordinateSystem string                `json:"coordinateSystem"`
	SourceComplete   bool                  `json:"sourceComplete"`
	Truncated        bool                  `json:"truncated"`
	SourceBytes      int64                 `json:"sourceBytes"`
	SourceLines      int                   `json:"sourceLines"`
	TotalLayers      int                   `json:"totalLayers"`
	TotalSegments    int                   `json:"totalSegments"`
	ReturnedLayers   int                   `json:"returnedLayers"`
	ReturnedSegments int                   `json:"returnedSegments"`
	OmittedSegments  int                   `json:"omittedSegments"`
	SupportSegments  int                   `json:"supportSegments"`
	BrimSegments     int                   `json:"brimSegments"`
	LayerSemantics   string                `json:"layerSemantics"`
	Warnings         []string              `json:"warnings"`
	Limits           SlicerToolpathLimits  `json:"limits"`
	Bounds           *SlicerToolpathBounds `json:"bounds,omitempty"`
	Layers           []SlicerToolpathLayer `json:"layers"`
}

type SlicerToolpathLimits struct {
	SourceBytes         int     `json:"sourceBytes"`
	LineBytes           int     `json:"lineBytes"`
	Lines               int     `json:"lines"`
	Layers              int     `json:"layers"`
	Segments            int     `json:"segments"`
	ArcSegmentsPerMove  int     `json:"arcSegmentsPerMove"`
	ArcSegmentsTotal    int     `json:"arcSegmentsTotal"`
	ArcChordToleranceMM float64 `json:"arcChordToleranceMm"`
}
type SlicerToolpathBounds struct {
	Min [3]float64 `json:"min"`
	Max [3]float64 `json:"max"`
}
type SlicerToolpathLayer struct {
	Index    int                     `json:"index"`
	Z        float64                 `json:"z"`
	Segments []SlicerToolpathSegment `json:"segments"`
}
type SlicerToolpathSegment struct {
	From      [3]float64 `json:"from"`
	To        [3]float64 `json:"to"`
	Extrusion float64    `json:"extrusion"`
	Feedrate  float64    `json:"feedrateMmSec,omitempty"`
	Feature   string     `json:"feature,omitempty"`
	Object    string     `json:"object,omitempty"`
	Tool      int        `json:"tool"`
}

// Marshal the Go zero-value default canonically, including legacy stored jobs.
func (r SlicerSliceRequest) MarshalJSON() ([]byte, error) {
	type wire SlicerSliceRequest
	if r.Quantity == 0 {
		r.Quantity = 1
	}
	return json.Marshal(wire(r))
}

// Omission defaults to one; explicit zero remains invalid at the HTTP boundary.
func (r *SlicerSliceRequest) UnmarshalJSON(data []byte) error {
	type wire SlicerSliceRequest
	value := wire{Quantity: 1}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if value.Quantity < 1 || value.Quantity > 512 {
		return fmt.Errorf("quantity must be an integer between 1 and 512")
	}
	*r = SlicerSliceRequest(value)
	return nil
}
func (r *SlicerPreviewRequest) UnmarshalJSON(data []byte) error {
	type wire SlicerPreviewRequest
	value := wire{Quantity: 1}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if value.Quantity < 1 || value.Quantity > 512 {
		return fmt.Errorf("quantity must be an integer between 1 and 512")
	}
	*r = SlicerPreviewRequest(value)
	return nil
}

func normalizeSlicerGeometry(quantity int, arrange, orient bool, rotate, rotateX, rotateY *float64) (int, error) {
	if quantity == 0 {
		quantity = 1
	}
	if quantity < 1 || quantity > 512 {
		return 0, fmt.Errorf("quantity must be an integer between 1 and 512")
	}
	if quantity > 1 && !arrange {
		return 0, fmt.Errorf("arrange must be true when quantity is greater than 1")
	}
	if orient && (rotate != nil || rotateX != nil || rotateY != nil) {
		return 0, fmt.Errorf("orient must be false when rotate, rotate_x or rotate_y is set")
	}
	for _, rotation := range []struct {
		name  string
		value *float64
	}{{"rotate", rotate}, {"rotate_x", rotateX}, {"rotate_y", rotateY}} {
		if rotation.value != nil && (math.IsNaN(*rotation.value) || math.IsInf(*rotation.value, 0)) {
			return 0, fmt.Errorf("%s must be finite", rotation.name)
		}
	}
	return quantity, nil
}

func writeSlicerGeometry(writer *multipart.Writer, quantity int, rotate, rotateX, rotateY *float64) {
	addFormField(writer, "quantity", strconv.Itoa(quantity))
	for _, rotation := range []struct {
		name  string
		value *float64
	}{{"rotate", rotate}, {"rotateX", rotateX}, {"rotateY", rotateY}} {
		if rotation.value != nil {
			addFormField(writer, rotation.name, strconv.FormatFloat(*rotation.value, 'g', -1, 64))
		}
	}
}

// SlicerUpstreamError retains the status and useful response text for synchronous callers.
// Background jobs deliberately do not persist raw upstream error bodies.
type SlicerUpstreamError struct {
	StatusCode int
	Message    string
}

func (e *SlicerUpstreamError) Error() string {
	return fmt.Sprintf("slicer error: %d %s", e.StatusCode, e.Message)
}
