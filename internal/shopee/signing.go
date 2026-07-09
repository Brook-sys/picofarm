package shopee

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// SignedRequest captures the deterministic Shopee V2 signing inputs and output.
// BaseString is exposed for tests/diagnostics only; do not log it in production
// when it contains token or shop context.
type SignedRequest struct {
	PartnerID   int64
	Timestamp   int64
	Path        string
	AccessToken string
	ShopID      int64
	BaseString  string
	Signature   string
}

// Signer signs Shopee Open Platform V2 requests with HMAC-SHA256.
type Signer struct {
	partnerID  int64
	partnerKey string
}

// NewSigner creates a signer for one Shopee partner app.
func NewSigner(partnerID int64, partnerKey string) Signer {
	return Signer{partnerID: partnerID, partnerKey: partnerKey}
}

// SignPublicRequest signs endpoints that are not scoped to a shop token.
func (s Signer) SignPublicRequest(path string, timestamp int64) SignedRequest {
	base := fmt.Sprintf("%d%s%d", s.partnerID, path, timestamp)
	return s.sign(path, timestamp, "", 0, base)
}

// SignShopRequest signs shop-scoped endpoints with access token and shop ID.
func (s Signer) SignShopRequest(path string, timestamp int64, accessToken string, shopID int64) SignedRequest {
	base := fmt.Sprintf("%d%s%d%s%d", s.partnerID, path, timestamp, accessToken, shopID)
	return s.sign(path, timestamp, accessToken, shopID, base)
}

func (s Signer) sign(path string, timestamp int64, accessToken string, shopID int64, base string) SignedRequest {
	mac := hmac.New(sha256.New, []byte(s.partnerKey))
	_, _ = mac.Write([]byte(base))
	return SignedRequest{
		PartnerID:   s.partnerID,
		Timestamp:   timestamp,
		Path:        path,
		AccessToken: accessToken,
		ShopID:      shopID,
		BaseString:  base,
		Signature:   hex.EncodeToString(mac.Sum(nil)),
	}
}

// SanitizeSignedURL redacts sensitive Shopee query parameters while preserving
// routing and non-secret debugging context.
func SanitizeSignedURL(raw string) string {
	sanitized := raw
	for _, key := range []string{"sign", "access_token", "refresh_token", "code"} {
		sanitized = redactQueryValue(sanitized, key)
	}
	return sanitized
}

func redactQueryValue(raw, key string) string {
	patterns := []string{"?" + key + "=", "&" + key + "="}
	for _, pattern := range patterns {
		start := 0
		for {
			idx := strings.Index(raw[start:], pattern)
			if idx < 0 {
				break
			}
			valueStart := start + idx + len(pattern)
			valueEnd := valueStart + strings.Index(raw[valueStart:]+"&", "&")
			raw = raw[:valueStart] + "[REDACTED]" + raw[valueEnd:]
			start = valueStart + len("[REDACTED]")
		}
	}
	return raw
}
