package types

import "time"

// Currency represents a currency code (ISO 4217)
type Currency string

const (
	NGN Currency = "NGN"
	USD Currency = "USD"
	GHS Currency = "GHS"
	KES Currency = "KES"
	ZAR Currency = "ZAR"
	GBP Currency = "GBP"
	EUR Currency = "EUR"
)

// Money represents a monetary amount with currency
type Money struct {
	Amount   int64    `json:"amount"`   // Amount in smallest currency unit (kobo, cents)
	Currency Currency `json:"currency"`
}

// ToMajorUnits converts to major currency units (Naira, Dollars)
func (m Money) ToMajorUnits() float64 {
	return float64(m.Amount) / 100
}

// FromMajorUnits creates Money from major currency units
func FromMajorUnits(amount float64, currency Currency) Money {
	return Money{
		Amount:   int64(amount * 100),
		Currency: currency,
	}
}

// IsZero returns true if the amount is zero
func (m Money) IsZero() bool {
	return m.Amount == 0
}

// Add returns a new Money with the sum of both amounts
func (m Money) Add(other Money) Money {
	return Money{
		Amount:   m.Amount + other.Amount,
		Currency: m.Currency,
	}
}

// TransactionStatus represents the status of a transaction
type TransactionStatus string

const (
	StatusPending   TransactionStatus = "pending"
	StatusSuccess   TransactionStatus = "success"
	StatusFailed    TransactionStatus = "failed"
	StatusCancelled TransactionStatus = "cancelled"
	StatusRefunded  TransactionStatus = "refunded"
)

// ProviderName identifies a payment provider
type ProviderName string

const (
	ProviderPaystack    ProviderName = "paystack"
	ProviderFlutterwave ProviderName = "flutterwave"
	ProviderMonnify     ProviderName = "monnify"
	ProviderStripe      ProviderName = "stripe"
)

// Metadata represents arbitrary key-value metadata
type Metadata map[string]interface{}

// CustomerInfo contains customer details
type CustomerInfo struct {
	ID        string `json:"id,omitempty"`
	Email     string `json:"email"`
	Phone     string `json:"phone,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}

// FullName returns the customer's full name
func (c *CustomerInfo) FullName() string {
	if c.FirstName == "" && c.LastName == "" {
		return ""
	}
	if c.FirstName == "" {
		return c.LastName
	}
	if c.LastName == "" {
		return c.FirstName
	}
	return c.FirstName + " " + c.LastName
}

// PaymentMethodType specifies the type of payment method
type PaymentMethodType string

const (
	PaymentMethodCard         PaymentMethodType = "card"
	PaymentMethodBank         PaymentMethodType = "bank"
	PaymentMethodUSSD         PaymentMethodType = "ussd"
	PaymentMethodTransfer     PaymentMethodType = "transfer"
	PaymentMethodMobileMoney  PaymentMethodType = "mobile_money"
	PaymentMethodQR           PaymentMethodType = "qr"
)

// PaymentMethod specifies payment method details
type PaymentMethod struct {
	Type      PaymentMethodType `json:"type"`
	CardToken string            `json:"card_token,omitempty"`
	BankCode  string            `json:"bank_code,omitempty"`
}

// Bank represents a bank
type Bank struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Code      string `json:"code"`
	Country   string `json:"country"`
	Currency  string `json:"currency"`
	IsActive  bool   `json:"is_active"`
}

// AccountInfo represents resolved bank account information
type AccountInfo struct {
	AccountNumber string `json:"account_number"`
	AccountName   string `json:"account_name"`
	BankCode      string `json:"bank_code"`
	BankName      string `json:"bank_name"`
}

// Timestamps provides common timestamp fields
type Timestamps struct {
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}
