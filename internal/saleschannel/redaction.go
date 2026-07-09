package saleschannel

import "regexp"

var sensitiveErrorPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(access_token\s*[=:]\s*)[^\s&"'{}]+`),
	regexp.MustCompile(`(?i)(refresh_token\s*[=:]\s*)[^\s&"'{}]+`),
	regexp.MustCompile(`(?i)(client_secret\s*[=:]\s*)[^\s&"'{}]+`),
	regexp.MustCompile(`(?i)(api_key\s*[=:]\s*)[^\s&"'{}]+`),
	regexp.MustCompile(`(?i)(partner_key\s*[=:]\s*)[^\s&"'{}]+`),
	regexp.MustCompile(`(?i)(token\s*[=:]\s*)[^\s&"'{}]+`),
	regexp.MustCompile(`(?i)(sign\s*[=:]\s*)[^\s&"'{}]+`),
	regexp.MustCompile(`(?i)(code\s*[=:]\s*)[^\s&"'{}]+`),
	regexp.MustCompile(`(?i)(Bearer\s+)[A-Za-z0-9._\-]+`),
}

// SanitizeErrorMessage removes credential-like values from provider/API errors
// before storing them in sync-run observability or returning them to clients.
func SanitizeErrorMessage(message string) string {
	if message == "" {
		return ""
	}
	sanitized := message
	for _, pattern := range sensitiveErrorPatterns {
		sanitized = pattern.ReplaceAllString(sanitized, `${1}[REDACTED]`)
	}
	return sanitized
}
