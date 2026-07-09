package shopee

import "testing"

func TestSignShopRequestBuildsDeterministicHMAC(t *testing.T) {
	signer := NewSigner(123456, "test-partner-key")

	signed := signer.SignShopRequest("/api/v2/order/get_order_list", 1700000000, "test-access-token", 987654321)

	if signed.PartnerID != 123456 {
		t.Fatalf("partner id: expected 123456, got %d", signed.PartnerID)
	}
	if signed.Timestamp != 1700000000 {
		t.Fatalf("timestamp: expected 1700000000, got %d", signed.Timestamp)
	}
	if signed.ShopID != 987654321 {
		t.Fatalf("shop id: expected 987654321, got %d", signed.ShopID)
	}
	if signed.BaseString != "123456/api/v2/order/get_order_list1700000000test-access-token987654321" {
		t.Fatalf("unexpected base string: %q", signed.BaseString)
	}
	if signed.Signature != "149ce3ff357e3f7ce1e8c53fddaffd20174e29bd9747bd55ec1fee27199f46f3" {
		t.Fatalf("unexpected signature: %q", signed.Signature)
	}
}

func TestSignPublicRequestOmitsTokenAndShop(t *testing.T) {
	signer := NewSigner(123456, "test-partner-key")

	signed := signer.SignPublicRequest("/api/v2/shop/auth_partner", 1700000000)

	if signed.BaseString != "123456/api/v2/shop/auth_partner1700000000" {
		t.Fatalf("unexpected base string: %q", signed.BaseString)
	}
	if signed.AccessToken != "" || signed.ShopID != 0 {
		t.Fatalf("public request should not include token/shop context: %+v", signed)
	}
	if signed.Signature != "cf285c1fa6f988838a5f051a716736354a8941a3a560fc4836d32d31c98be106" {
		t.Fatalf("unexpected signature: %q", signed.Signature)
	}
}

func TestSanitizeSignedURLRedactsShopeeSecrets(t *testing.T) {
	input := "https://partner.shopeemobile.com/api/v2/order/get_order_list?partner_id=123456&timestamp=1700000000&sign=abcdef&access_token=secret-token&shop_id=987654321&code=oauth-code"

	got := SanitizeSignedURL(input)

	want := "https://partner.shopeemobile.com/api/v2/order/get_order_list?partner_id=123456&timestamp=1700000000&sign=[REDACTED]&access_token=[REDACTED]&shop_id=987654321&code=[REDACTED]"
	if got != want {
		t.Fatalf("expected sanitized URL %q, got %q", want, got)
	}
}
