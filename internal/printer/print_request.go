package printer

import (
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/textproto"
	"net/url"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxRemotePrintFolderLength = 255

// PrintRequest describes one local file and its requested remote destination.
type PrintRequest struct {
	Filename        string
	LocalPath       string
	RemoteDirectory string
}

func createRemoteFilePart(writer *multipart.Writer, fieldName, remoteName string) (io.Writer, error) {
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{
		"name":     fieldName,
		"filename": remoteName,
	}))
	header.Set("Content-Type", "application/octet-stream")
	return writer.CreatePart(header)
}

// NormalizeRemotePrintFolder canonicalizes a POSIX path relative to a printer's
// G-code root. An empty value means the protocol's default root.
func NormalizeRemotePrintFolder(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if utf8.RuneCountInString(value) > maxRemotePrintFolderLength {
		return "", fmt.Errorf("print folder must be at most %d characters", maxRemotePrintFolderLength)
	}
	if strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return "", fmt.Errorf("print folder must not contain control characters")
	}
	if strings.Contains(value, `\`) {
		return "", fmt.Errorf("print folder must use forward slashes")
	}
	if err := rejectEncodedPathMetacharacters(value); err != nil {
		return "", err
	}
	if parsed, err := url.Parse(value); err == nil && parsed.Scheme != "" {
		return "", fmt.Errorf("print folder must be a relative path")
	}

	for _, segment := range strings.Split(value, "/") {
		if segment == "." || segment == ".." {
			return "", fmt.Errorf("print folder must not contain %q segments", segment)
		}
	}

	normalized := strings.TrimPrefix(path.Clean("/"+value), "/")
	if normalized == "" || normalized == "." {
		return "", fmt.Errorf("print folder must name a subdirectory")
	}
	return normalized, nil
}

func rejectEncodedPathMetacharacters(value string) error {
	current := value
	for range 3 {
		decoded, err := url.PathUnescape(current)
		if err != nil {
			return fmt.Errorf("print folder contains an invalid percent escape")
		}
		if decoded == current {
			return nil
		}
		if strings.Count(decoded, "/") != strings.Count(current, "/") || strings.Contains(decoded, `\`) {
			return fmt.Errorf("print folder must not encode path separators")
		}
		if strings.IndexFunc(decoded, unicode.IsControl) >= 0 {
			return fmt.Errorf("print folder must not encode control characters")
		}
		for _, segment := range strings.Split(decoded, "/") {
			if segment == "." || segment == ".." {
				return fmt.Errorf("print folder must not encode %q segments", segment)
			}
		}
		current = decoded
	}
	return fmt.Errorf("print folder contains excessive percent encoding")
}
