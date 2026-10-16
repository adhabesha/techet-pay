package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Money is stored in minor units (santim). 1 ETB = 100 santim.
// It marshals as a decimal string ("125.50") so JSON never loses precision.
type Money int64

func (m Money) String() string {
	neg := ""
	v := int64(m)
	if v < 0 {
		neg = "-"
		v = -v
	}
	return fmt.Sprintf("%s%d.%02d", neg, v/100, v%100)
}

func (m Money) MarshalJSON() ([]byte, error) { return []byte(`"` + m.String() + `"`), nil }

// UnmarshalJSON accepts a leading "-" because ledger balances (and therefore the
// saved snapshot) can legitimately be negative. API handlers range-check every
// amount (MinAmount..MaxAmount), so a negative request amount is still rejected.
func (m *Money) UnmarshalJSON(b []byte) error {
	s := strings.Trim(strings.TrimSpace(string(b)), `"`)
	neg := strings.HasPrefix(s, "-")
	v, err := parseMoney(strings.TrimPrefix(s, "-"))
	if err != nil {
		return err
	}
	if neg {
		v = -v
	}
	*m = v
	return nil
}

func parseMoney(s string) (Money, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "null" {
		return 0, errors.New("amount is required")
	}
	parts := strings.Split(s, ".")
	if len(parts) > 2 {
		return 0, errors.New("invalid amount")
	}
	whole, frac := parts[0], ""
	if len(parts) == 2 {
		frac = parts[1]
	}
	if whole == "" || len(whole) > 9 || len(frac) > 2 {
		return 0, errors.New("invalid amount (max 2 decimals)")
	}
	for _, c := range whole + frac {
		if c < '0' || c > '9' {
			return 0, errors.New("invalid amount")
		}
	}
	for len(frac) < 2 {
		frac += "0"
	}
	w, _ := strconv.ParseInt(whole, 10, 64)
	f, _ := strconv.ParseInt(frac, 10, 64)
	return Money(w*100 + f), nil
}

const (
	StatusPending = "pending"
	StatusSuccess = "success"
	StatusFailed  = "failed"

	MinAmount = Money(100)          // 1.00 ETB
	MaxAmount = Money(1_000_000_00) // 1,000,000.00 ETB per transaction
)

type Merchant struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Email         string    `json:"email"`
	PassSalt      string    `json:"pass_salt"`
	PassHash      string    `json:"pass_hash"`
	PublicKey     string    `json:"public_key"`
	SecretHash    string    `json:"secret_hash"`
	SecretHint    string    `json:"secret_hint"`
	WebhookURL    string    `json:"webhook_url"`
	WebhookSecret string    `json:"webhook_secret"`
	CreatedAt     time.Time `json:"created_at"`
}

type Transaction struct {
	ID            string    `json:"id"`
	CheckoutToken string    `json:"checkout_token"`
	MerchantID    string    `json:"merchant_id"`
	TxRef         string    `json:"tx_ref"`
	Amount        Money     `json:"amount"`
	Fee           Money     `json:"fee"`
	Currency      string    `json:"currency"`
	Email         string    `json:"email"`
	FirstName     string    `json:"first_name"`
	LastName      string    `json:"last_name"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	ReturnURL     string    `json:"return_url"`
	Method        string    `json:"method"`
	Account       string    `json:"account"` // masked payer account (phone / card last4)
	Status        string    `json:"status"`
	Stage         string    `json:"stage"` // "", "otp"
	OTPAttempts   int       `json:"otp_attempts"`
	FailReason    string    `json:"fail_reason"`
	ProviderRef   string    `json:"provider_ref"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	ExpiresAt     time.Time `json:"expires_at"`
}

type LedgerEntry struct {
	ID        int64     `json:"id"`
	TxID      string    `json:"tx_id"`
	Account   string    `json:"account"`
	Debit     Money     `json:"debit"`
	Credit    Money     `json:"credit"`
	Memo      string    `json:"memo"`
	CreatedAt time.Time `json:"created_at"`
}

type Transfer struct {
	ID            string    `json:"id"`
	MerchantID    string    `json:"merchant_id"`
	Reference     string    `json:"reference"`
	AccountName   string    `json:"account_name"`
	AccountNumber string    `json:"account_number"`
	BankCode      string    `json:"bank_code"`
	Amount        Money     `json:"amount"`
	Fee           Money     `json:"fee"`
	Currency      string    `json:"currency"`
	Status        string    `json:"status"`
	FailReason    string    `json:"fail_reason"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Delivery struct {
	ID          string    `json:"id"`
	MerchantID  string    `json:"merchant_id"`
	Event       string    `json:"event"`
	URL         string    `json:"url"`
	Secret      string    `json:"secret"`
	Payload     string    `json:"payload"`
	Attempts    int       `json:"attempts"`
	Status      string    `json:"status"` // pending, delivered, failed
	LastCode    int       `json:"last_code"`
	LastError   string    `json:"last_error"`
	NextAttempt time.Time `json:"next_attempt"`
	CreatedAt   time.Time `json:"created_at"`
}

type Session struct {
	MerchantID string    `json:"merchant_id"`
	Expires    time.Time `json:"expires"`
}

type Config struct {
	BaseURL              string
	DataPath             string
	FeeBPS               int64 // platform fee in basis points (250 = 2.5%)
	PayoutFee            Money
	AllowPrivateWebhooks bool
	PayoutDelay          time.Duration
	TxTTL                time.Duration
	Backoff              []time.Duration
}
