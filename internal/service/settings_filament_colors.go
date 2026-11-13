package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// FilamentTypeColorsSettingKey is the settings key that stores the map of
// filament type to label color shown on the G-code files screen. The value is
// a JSON object such as
//
//	{"pla":"#22c55e","petg":"#3b82f6","tpu":"#ec4899"}
//
// Keys are normalized through the same rules as gcode_files.material_type
// (see normalizeMaterialType), so an entry written as "PLA-CF" is stored as
// "pla" — which is the value the G-code parser actually produces for that
// filament.
const FilamentTypeColorsSettingKey = "filament_type_colors"

// ErrInvalidFilamentTypeColors is returned when the value for
// FilamentTypeColorsSettingKey is not a JSON object mapping filament types to
// hex colors. The settings API is a generic key/value store with no schema of
// its own, so a caller writing garbage would otherwise reach the front end and
// only fail there (the badge would silently fall back to a neutral color).
var ErrInvalidFilamentTypeColors = errors.New("invalid filament type colors")

// filamentColorPattern accepts 3 and 6 digit hex colors with a leading '#',
// which is what <input type="color"> produces and what the badge renderer parses.
var filamentColorPattern = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

// validateFilamentTypeColors validates the filament color setting and returns
// the value to persist. An empty value clears the setting (the front end then
// uses its built-in defaults).
//
// The value is rewritten (valid only) rather than rejected when it merely needs
// normalization, because the stored keys must match the normalized
// material_type values that arrive from the G-code parser: a mismatch would
// make a configured color silently invisible on the files screen.
func validateFilamentTypeColors(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	if trimmed[0] != '{' {
		return "", fmt.Errorf("%w: expected a JSON object of filament type to hex color", ErrInvalidFilamentTypeColors)
	}

	var raw map[string]string
	if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidFilamentTypeColors, err)
	}

	normalized := make(map[string]string, len(raw))
	for key, color := range raw {
		filamentType := normalizeMaterialType(key)
		if filamentType == "" {
			return "", fmt.Errorf("%w: %q does not name a filament type", ErrInvalidFilamentTypeColors, key)
		}
		color = strings.ToLower(strings.TrimSpace(color))
		if !filamentColorPattern.MatchString(color) {
			return "", fmt.Errorf("%w: color for %q must be a hex value like #RRGGBB, got %q", ErrInvalidFilamentTypeColors, key, color)
		}
		// Two entries can collapse into the same normalized key ("PLA" and
		// "PLA-CF" both become "pla"). Rejecting that keeps the stored value
		// deterministic instead of depending on map iteration order.
		if existing, ok := normalized[filamentType]; ok && existing != color {
			return "", fmt.Errorf("%w: %q conflicts with another entry that also normalizes to %q", ErrInvalidFilamentTypeColors, key, filamentType)
		}
		normalized[filamentType] = color
	}

	encoded, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidFilamentTypeColors, err)
	}
	return string(encoded), nil
}
