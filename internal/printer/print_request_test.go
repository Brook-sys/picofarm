package printer

import (
	"strings"
	"testing"
)

func TestNormalizeRemotePrintFolder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "empty uses root", input: "", want: ""},
		{name: "leading slash", input: "/sda1", want: "sda1"},
		{name: "trailing slash", input: "sda1/", want: "sda1"},
		{name: "duplicate slashes", input: "//jobs/a//", want: "jobs/a"},
		{name: "spaces", input: "jobs 2026", want: "jobs 2026"},
		{name: "current directory", input: ".", wantErr: true},
		{name: "parent directory", input: "..", wantErr: true},
		{name: "traversal", input: "sda1/../x", wantErr: true},
		{name: "windows path", input: `C:\\prints`, wantErr: true},
		{name: "url", input: "http://host/path", wantErr: true},
		{name: "nested traversal", input: "gcodes/../../config", wantErr: true},
		{name: "encoded parent segment", input: "%2e%2e/config", wantErr: true},
		{name: "encoded slash", input: "sda1%2fconfig", wantErr: true},
		{name: "encoded backslash", input: "sda1%5cconfig", wantErr: true},
		{name: "null byte", input: "sda1\x00config", wantErr: true},
		{name: "line feed", input: "sda1\nconfig", wantErr: true},
		{name: "carriage return", input: "sda1\rconfig", wantErr: true},
		{name: "delete control", input: "sda1\x7fconfig", wantErr: true},
		{name: "too long", input: strings.Repeat("a", maxRemotePrintFolderLength+1), wantErr: true},
		{name: "maximum length", input: strings.Repeat("a", maxRemotePrintFolderLength), want: strings.Repeat("a", maxRemotePrintFolderLength)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := NormalizeRemotePrintFolder(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("NormalizeRemotePrintFolder(%q) expected error, got %q", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeRemotePrintFolder(%q): %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("NormalizeRemotePrintFolder(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
