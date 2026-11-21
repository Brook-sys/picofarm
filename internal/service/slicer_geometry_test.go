package service

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"testing"
)

func TestSlicerGeometryJSONContract(t *testing.T) {
	for _, body := range []string{`{}`, `{"quantity":1}`, `{"quantity":512,"arrange":true,"rotate_x":0,"rotate_y":-90,"rotate":360}`} {
		var slice SlicerSliceRequest
		var preview SlicerPreviewRequest
		if err := json.Unmarshal([]byte(body), &slice); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(body), &preview); err != nil {
			t.Fatal(err)
		}
		if slice.Quantity < 1 || slice.Quantity != preview.Quantity {
			t.Fatal("default or quantity lost")
		}
		if _, err := normalizeSlicerGeometry(slice.Quantity, slice.Arrange, slice.Orient, slice.Rotate, slice.RotateX, slice.RotateY); err != nil {
			t.Fatal(err)
		}
	}
	for _, body := range []string{`{"quantity":0}`, `{"quantity":-1}`, `{"quantity":513}`, `{"quantity":1.5}`, `{"quantity":"2"}`, `{"rotate":1e999}`} {
		var slice SlicerSliceRequest
		var preview SlicerPreviewRequest
		if json.Unmarshal([]byte(body), &slice) == nil || json.Unmarshal([]byte(body), &preview) == nil {
			t.Errorf("accepted %s", body)
		}
	}
}

func TestSlicerGeometryRejectsBeforeLookup(t *testing.T) {
	zero, nan, inf := 0.0, math.NaN(), math.Inf(1)
	for _, req := range []SlicerSliceRequest{
		{Quantity: -1}, {Quantity: 513}, {Quantity: 2},
		{Orient: true, Rotate: &zero}, {Orient: true, RotateX: &zero}, {Orient: true, RotateY: &zero},
		{Rotate: &nan}, {RotateX: &inf}, {RotateY: &nan},
	} {
		s := &SlicerService{}
		if _, err := s.SliceSTL(context.Background(), req); err == nil {
			t.Errorf("accepted %+v", req)
		}
		if _, err := s.PreviewSTL(context.Background(), SlicerPreviewRequest{Quantity: req.Quantity, Orient: req.Orient, Rotate: req.Rotate, RotateX: req.RotateX, RotateY: req.RotateY}); err == nil {
			t.Errorf("preview accepted %+v", req)
		}
	}
}

func TestSlicerJobsGeometryPersistsAndDeduplicates(t *testing.T) {
	settings := &jobMemorySettings{}
	s, err := newSlicerJobsService(context.Background(), settings, func(ctx context.Context, _ SlicerSliceRequest) (*SlicerSliceResult, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	angle := 0.0
	req := jobRequest()
	req.Quantity, req.Arrange, req.RotateX = 2, true, &angle
	job, err := s.Submit(req, "geometry")
	if err != nil {
		t.Fatal(err)
	}
	angle = 90
	stored, err := s.Get(job.ID)
	if err != nil || stored.Request.Quantity != 2 || *stored.Request.RotateX != 0 {
		t.Fatalf("stored %+v %v", stored, err)
	}
	if _, err := s.Submit(req, "geometry"); !errors.Is(err, ErrSlicerJobInvalid) {
		t.Fatalf("changed rotation: %v", err)
	}
	req.RotateX = nil
	req.Quantity = 3
	if _, err := s.Submit(req, "geometry"); !errors.Is(err, ErrSlicerJobInvalid) {
		t.Fatalf("changed quantity: %v", err)
	}
	req.Arrange = false
	if _, err := s.Submit(req, "invalid"); !errors.Is(err, ErrSlicerJobInvalid) {
		t.Fatalf("invalid geometry: %v", err)
	}
}
