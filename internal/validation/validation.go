// Package validation provides input validation utilities for API handlers.
package validation

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// ValidationError represents a collection of validation errors.
type ValidationError struct {
	Errors []FieldError `json:"errors"`
}

// FieldError represents a validation error for a specific field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	if len(e.Errors) == 0 {
		return "validation failed"
	}
	msgs := make([]string, len(e.Errors))
	for i, fe := range e.Errors {
		msgs[i] = fmt.Sprintf("%s: %s", fe.Field, fe.Message)
	}
	return strings.Join(msgs, "; ")
}

// Validator collects validation errors.
type Validator struct {
	errors []FieldError
}

// New creates a new Validator.
func New() *Validator {
	return &Validator{
		errors: make([]FieldError, 0),
	}
}

// AddError adds a validation error for a field.
func (v *Validator) AddError(field, message string) {
	v.errors = append(v.errors, FieldError{Field: field, Message: message})
}

// HasErrors returns true if there are validation errors.
func (v *Validator) HasErrors() bool {
	return len(v.errors) > 0
}

// Error returns the validation error if there are errors, nil otherwise.
func (v *Validator) Error() error {
	if !v.HasErrors() {
		return nil
	}
	return &ValidationError{Errors: v.errors}
}

// Required validates that a string field is not empty.
func (v *Validator) Required(field, value string) {
	if strings.TrimSpace(value) == "" {
		v.AddError(field, "is required")
	}
}

// MaxLength validates that a string doesn't exceed max length (in runes).
func (v *Validator) MaxLength(field, value string, max int) {
	if utf8.RuneCountInString(value) > max {
		v.AddError(field, fmt.Sprintf("must be at most %d characters", max))
	}
}

// NonNegative validates that an int is non-negative (>= 0).
func (v *Validator) NonNegative(field string, value int) {
	if value < 0 {
		v.AddError(field, "must not be negative")
	}
}

// NonNegativeFloat validates that a float64 is non-negative (>= 0).
func (v *Validator) NonNegativeFloat(field string, value float64) {
	if value < 0 {
		v.AddError(field, "must not be negative")
	}
}

// NoControlChars validates that a string doesn't contain control characters (except common whitespace).
func (v *Validator) NoControlChars(field, value string) {
	for _, r := range value {
		if r < 32 && r != '\t' && r != '\n' && r != '\r' {
			v.AddError(field, "contains invalid control characters")
			return
		}
	}
}
