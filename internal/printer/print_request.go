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
)

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
	if strings.Contains(value, `\`) {
		return "", fmt.Errorf("print folder must use forward slashes")
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
