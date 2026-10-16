package main

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

// Payment methods supported by the sandbox. In production each code maps to a
// real integration behind the same interface (see Provider below).
type Method struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Kind string `json:"kind"` // wallet | card
}

var methods = []Method{
	{"telebirr", "Telebirr", "wallet"},
	{"cbebirr", "CBE Birr", "wallet"},
	{"mpesa", "M-Pesa", "wallet"},
	{"amole", "Amole", "wallet"},
	{"card", "Debit / Credit card", "card"},
}

func methodByCode(code string) (Method, bool) {
	for _, m := range methods {
		if m.Code == code {
			return m, true
		}
	}
	return Method{}, false
}

type PayInput struct {
	Method     string `json:"method"`
	Phone      string `json:"phone"`
	CardNumber string `json:"card_number"`
	Expiry     string `json:"expiry"`
	CVC        string `json:"cvc"`
}

// Provider is the seam where real rails (Telebirr API, bank switch, card
// processor) plug in. The sandbox implementation is SandboxProvider.
type Provider interface {
	// Charge starts a payment. needOTP=true means the payer must confirm.
	Charge(m Method, in PayInput) (account string, needOTP bool, err error)
	ConfirmOTP(otp string) bool
	// Payout validates a destination and decides the simulated outcome.
	ValidateDestination(bank Bank, account string) error
	PayoutOutcome(account string) (ok bool, reason string)
}

type SandboxProvider struct{}

const sandboxOTP = "123456"

var phoneRe = regexp.MustCompile(`^0[79]\d{8}$`)

func normalizePhone(p string) (string, bool) {
	p = strings.NewReplacer(" ", "", "-", "").Replace(p)
	switch {
	case strings.HasPrefix(p, "+251"):
		p = "0" + p[4:]
	case strings.HasPrefix(p, "251"):
		p = "0" + p[3:]
	}
	return p, phoneRe.MatchString(p)
}

func luhn(num string) bool {
	sum, alt := 0, false
	for i := len(num) - 1; i >= 0; i-- {
		d := int(num[i] - '0')
		if alt {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		alt = !alt
	}
	return sum%10 == 0
}

func digitsOnly(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func (SandboxProvider) Charge(m Method, in PayInput) (string, bool, error) {
	switch m.Kind {
	case "wallet":
		phone, ok := normalizePhone(in.Phone)
		if !ok {
			return "", false, errors.New("enter a valid Ethiopian mobile number (e.g. 0911223344)")
		}
		if strings.HasSuffix(phone, "0000") { // sandbox rule: simulates an empty wallet
			return phone[:4] + "****" + phone[len(phone)-2:], false, errors.New("insufficient wallet balance")
		}
		return phone[:4] + "****" + phone[len(phone)-2:], true, nil
	case "card":
		num := strings.NewReplacer(" ", "", "-", "").Replace(in.CardNumber)
		if !digitsOnly(num) || len(num) < 13 || len(num) > 19 || !luhn(num) {
			return "", false, errors.New("invalid card number")
		}
		exp, err := time.Parse("01/06", strings.TrimSpace(in.Expiry))
		if err != nil || exp.AddDate(0, 1, 0).Before(time.Now()) {
			return "", false, errors.New("card expiry is invalid or in the past (use MM/YY)")
		}
		if !digitsOnly(in.CVC) || len(in.CVC) < 3 || len(in.CVC) > 4 {
			return "", false, errors.New("invalid security code")
		}
		last4 := "•••• " + num[len(num)-4:]
		if num == "4000000000000002" { // sandbox rule: always declined
			return last4, false, errors.New("card declined by issuer")
		}
		return last4, true, nil
	}
	return "", false, errors.New("unsupported payment method")
}

func (SandboxProvider) ConfirmOTP(otp string) bool { return strings.TrimSpace(otp) == sandboxOTP }

// ---- payout destinations ----

type Bank struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Kind   string `json:"kind"` // bank | wallet
	Digits string `json:"-"`
}

var banks = []Bank{
	{"cbe", "Commercial Bank of Ethiopia", "bank", "13"},
	{"awash", "Awash Bank", "bank", "14"},
	{"dashen", "Dashen Bank", "bank", "13"},
	{"abyssinia", "Bank of Abyssinia", "bank", "8,16"},
	{"telebirr", "Telebirr wallet", "wallet", "phone"},
	{"cbebirr", "CBE Birr wallet", "wallet", "phone"},
}

func bankByCode(code string) (Bank, bool) {
	for _, b := range banks {
		if b.Code == code {
			return b, true
		}
	}
	return Bank{}, false
}

func (SandboxProvider) ValidateDestination(b Bank, account string) error {
	if b.Digits == "phone" {
		if _, ok := normalizePhone(account); !ok {
			return errors.New("account_number must be a valid Ethiopian mobile number for wallet payouts")
		}
		return nil
	}
	if !digitsOnly(account) || len(account) < 8 || len(account) > 16 {
		return errors.New("account_number must be 8-16 digits")
	}
	return nil
}

func (SandboxProvider) PayoutOutcome(account string) (bool, string) {
	if strings.HasSuffix(account, "0000") { // sandbox rule: bank rejects the account
		return false, "destination account rejected by bank"
	}
	return true, ""
}
