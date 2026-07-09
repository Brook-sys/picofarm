package shopee

import "time"

// Shop is the minimal Shopee account metadata needed by the provider-neutral adapter.
type Shop struct {
	ID   int64
	Name string
}

// Order is a normalized Shopee order/detail payload returned by a fakeable client.
type Order struct {
	SN          string
	Status      string
	Currency    string
	TotalAmount float64
	BuyerName   string
	Recipient   string
	ShippingID  string
	RawJSON     string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	OrderItems  []OrderItem
}

// OrderItem is a normalized Shopee line item, including item/model identifiers and SKUs.
type OrderItem struct {
	ItemID    int64
	ModelID   int64
	ItemSKU   string
	ModelSKU  string
	Title     string
	Quantity  int
	UnitPrice float64
	Currency  string
}

// Item is a normalized Shopee product/item with optional model variants.
type Item struct {
	ID        int64
	Title     string
	Status    string
	URL       string
	Price     float64
	Currency  string
	SKU       string
	Stock     *int
	ImageURL  string
	RawJSON   string
	ModelList []Model
}

// Model is a Shopee item variation/model.
type Model struct {
	ID       int64
	SKU      string
	Title    string
	Price    float64
	Currency string
	Stock    *int
}
