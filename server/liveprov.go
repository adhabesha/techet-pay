package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Live payment adapters
//
// Sandbox stays available to every merchant, always. A merchant only reaches a
// live adapter when BOTH are true:
//   1. the owner switched that merchant to "live access" in the admin panel, and
//   2. the provider is in "live" mode, fully configured, with an implemented adapter.
// A live merchant is never silently downgraded to sandbox (and vice versa).
//
// To add a real provider: open its provider_<name>.go file, fill in Charge,
// ParseCallback and Check, set implemented() to true, then enter the
// credentials in the admin panel (Providers page).
// ---------------------------------------------------------------------------

// CredField describes one credential the provider gives you after the contract.
type CredField struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	Hint      string `json:"hint"`
	Secret    bool   `json:"secret"`
	Required  bool   `json:"required"`
	Multiline bool   `json:"multiline"`
}

type Creds map[string]string

type LiveCharge struct {
	TxID        string // our unique id: send this to the provider as the merchant reference
	TxRef       string // the merchant's own reference (informational)
	Amount      Money
	Currency    string
	Method      string
	Phone       string // normalised 09xxxxxxxx for wallet methods
	Email       string
	Description string
	CallbackURL string // where the provider must POST the result
	ReturnURL   string
}

type LiveChargeResult struct {
	Account     string // masked payer account to show on receipts
	ProviderRef string
	RedirectURL string // set when the payer must continue on the provider's page
}

type LivePayout struct {
	TransferID    string
	Reference     string
	Amount        Money
	Currency      string
	BankCode      string
	AccountName   string
	AccountNumber string
	CallbackURL   string
}

type LivePayoutResult struct {
	ProviderRef string
	Done        bool // true if the provider already confirmed the money was sent
}

// CallbackEvent is what a provider's server-to-server notification means.
type CallbackEvent struct {
	Kind        string // "payment" (default) or "payout"
	ID          string // our TxID / TransferID, as we sent it to the provider
	ProviderRef string
	Success     bool
	Amount      Money // optional but strongly recommended: checked against our record
	Reason      string
	Response    string // body to send back to the provider (default "ok")
}

// DeclineError means the provider definitively refused: the payment fails.
// Any other error is treated as "could not reach / unclear" and the payer may retry.
type DeclineError struct{ Reason string }

func (e *DeclineError) Error() string { return e.Reason }

var ErrNotImplemented = errors.New("this live adapter has not been implemented yet")

type LiveProvider interface {
	Code() string
	Name() string
	Kind() string // "payment" or "payout"
	Fields() []CredField
	// Implemented must return true only when Charge/ParseCallback/Check really work.
	Implemented() bool
	// Check makes a harmless authenticated call to prove the credentials work.
	Check(ctx context.Context, c Creds) error
	Charge(ctx context.Context, c Creds, req LiveCharge) (LiveChargeResult, error)
	// ParseCallback MUST verify the provider's signature and reject forgeries.
	ParseCallback(c Creds, r *http.Request) (CallbackEvent, error)
	Payout(ctx context.Context, c Creds, req LivePayout) (LivePayoutResult, error)
}

// stubAdapter is the starting point of every provider file: everything refuses
// to run until the real implementation replaces it.
type stubAdapter struct {
	code, name, kind string
	fields           []CredField
}

func (s stubAdapter) Code() string                       { return s.code }
func (s stubAdapter) Name() string                       { return s.name }
func (s stubAdapter) Kind() string                       { return s.kind }
func (s stubAdapter) Fields() []CredField                { return s.fields }
func (s stubAdapter) Implemented() bool                  { return false }
func (s stubAdapter) Check(context.Context, Creds) error { return ErrNotImplemented }
func (s stubAdapter) Charge(context.Context, Creds, LiveCharge) (LiveChargeResult, error) {
	return LiveChargeResult{}, ErrNotImplemented
}
func (s stubAdapter) ParseCallback(Creds, *http.Request) (CallbackEvent, error) {
	return CallbackEvent{}, ErrNotImplemented
}
func (s stubAdapter) Payout(context.Context, Creds, LivePayout) (LivePayoutResult, error) {
	return LivePayoutResult{}, ErrNotImplemented
}

var liveProviders = map[string]LiveProvider{}

func registerLive(p LiveProvider) { liveProviders[p.Code()] = p }

func liveProviderList() []LiveProvider {
	out := make([]LiveProvider, 0, len(liveProviders))
	for _, p := range liveProviders {
		out = append(out, p)
	}
	order := map[string]int{"telebirr": 1, "cbebirr": 2, "mpesa": 3, "amole": 4, "card": 5, "bank_payout": 6}
	sort.Slice(out, func(i, j int) bool { return order[out[i].Code()] < order[out[j].Code()] })
	return out
}

// Fields every adapter file can use: base address plus anything extra the provider requires.
func commonFields() []CredField {
	return []CredField{
		{Key: "extra_json", Label: "Extra settings (JSON)", Hint: "Optional: any additional values the provider's guide asks for", Multiline: true},
	}
}

// ---- provider configuration (stored encrypted) ----

type ProviderConfig struct {
	Code         string            `json:"code"`
	Mode         string            `json:"mode"`   // "sandbox" (default) or "live"
	Fields       map[string]string `json:"fields"` // values are sealed with the Vault
	UpdatedAt    time.Time         `json:"updated_at"`
	LastCheckAt  time.Time         `json:"last_check_at"`
	LastCheckOK  bool              `json:"last_check_ok"`
	LastCheckMsg string            `json:"last_check_msg"`
}

// credsLocked decrypts a provider's credentials. s.mu must be held.
func (a *App) credsLocked(code string) (Creds, error) {
	pc := a.s.d.Providers[code]
	if pc == nil {
		return nil, errors.New("provider is not configured")
	}
	if a.vault == nil {
		return nil, errors.New("TECHET_SECRET_KEY is not set on the server")
	}
	c := Creds{}
	for k, sealed := range pc.Fields {
		v, err := a.vault.Open(sealed)
		if err != nil {
			return nil, err
		}
		c[k] = v
	}
	return c, nil
}

func missingRequired(lp LiveProvider, pc *ProviderConfig) []string {
	var miss []string
	for _, f := range lp.Fields() {
		if f.Required && (pc == nil || pc.Fields[f.Key] == "") {
			miss = append(miss, f.Label)
		}
	}
	return miss
}

// routeLocked decides sandbox vs live for one merchant and method. s.mu must be held.
func (a *App) routeLocked(m *Merchant, method string) (LiveProvider, Creds, bool, error) {
	if m == nil || !m.LiveEnabled {
		return nil, nil, false, nil
	}
	lp, pc := liveProviders[method], a.s.d.Providers[method]
	if lp == nil || pc == nil || pc.Mode != "live" || !lp.Implemented() || len(missingRequired(lp, pc)) > 0 {
		return nil, nil, false, bad("this payment method is not available for live payments yet")
	}
	c, err := a.credsLocked(method)
	if err != nil {
		return nil, nil, false, &apiError{503, "payment provider is temporarily unavailable"}
	}
	return lp, c, true, nil
}

func (a *App) callbackURL(base, code string) string {
	if base == "" {
		base, _ = a.lastBase.Load().(string)
	}
	return strings.TrimRight(base, "/") + "/callbacks/" + code
}

// payLiveLocked starts a live payment. It is entered and left with s.mu held,
// but releases it during the network call to the provider.
func (a *App) payLiveLocked(t *Transaction, meth Method, in PayInput, lp LiveProvider, c Creds, base string) error {
	if t.Stage != "" {
		return conflict("this payment has already been started")
	}
	var phone string
	if meth.Kind == "wallet" {
		p, okp := normalizePhone(in.Phone)
		if !okp {
			return bad("enter a valid Ethiopian mobile number (e.g. 0911223344)")
		}
		phone = p
	}
	req := LiveCharge{TxID: t.ID, TxRef: t.TxRef, Amount: t.Amount, Currency: t.Currency, Method: meth.Code,
		Phone: phone, Email: t.Email, Description: t.Description,
		CallbackURL: a.callbackURL(base, meth.Code), ReturnURL: t.ReturnURL}
	t.Stage = "starting" // blocks a second click while the lock is released
	id := t.ID
	a.s.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	res, err := lp.Charge(ctx, c, req)
	cancel()
	a.s.mu.Lock()
	t = a.s.d.Txs[id]
	if t == nil || t.Status != StatusPending {
		return conflict("this payment is no longer pending")
	}
	if err != nil {
		var d *DeclineError
		if errors.As(err, &d) {
			t.Method, t.Mode = meth.Code, "live"
			a.failLocked(t, d.Reason)
			return nil
		}
		t.Stage = ""
		a.auditLocked("system", "payment.provider_error", meth.Code+" tx="+t.ID+": "+clipStr(err.Error(), 200), "")
		return &apiError{502, "the payment provider could not be reached, please try again"}
	}
	t.Method, t.Mode, t.Account, t.UpdatedAt = meth.Code, "live", res.Account, time.Now().UTC()
	if res.ProviderRef != "" {
		t.ProviderRef = res.ProviderRef
	}
	if res.RedirectURL != "" {
		t.Stage, t.RedirectURL = "redirect", res.RedirectURL
	} else {
		t.Stage = "waiting"
	}
	a.s.markDirty()
	return nil
}

// providerCallback receives the provider's server-to-server notification.
func (a *App) providerCallback(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("provider")
	lp := liveProviders[code]
	if lp == nil {
		http.NotFound(w, r)
		return
	}
	a.s.mu.Lock()
	pc := a.s.d.Providers[code]
	live := pc != nil && pc.Mode == "live" && lp.Implemented()
	var creds Creds
	var cerr error
	if live {
		creds, cerr = a.credsLocked(code)
	}
	a.s.mu.Unlock()
	if !live || cerr != nil {
		http.NotFound(w, r)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	ev, err := lp.ParseCallback(creds, r)
	if err != nil {
		_, _ = io.Copy(io.Discard, r.Body)
		a.audit(r, "callback.rejected", code+": "+clipStr(err.Error(), 200))
		http.Error(w, "rejected", http.StatusBadRequest)
		return
	}
	a.s.mu.Lock()
	msg := a.applyEventLocked(code, ev)
	a.s.mu.Unlock()
	a.audit(r, "callback.accepted", code+" "+msg)
	resp := ev.Response
	if resp == "" {
		resp = "ok"
	}
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(resp))
}

func (a *App) applyEventLocked(code string, ev CallbackEvent) string {
	if ev.Kind == "payout" {
		t := a.s.d.Transfers[ev.ID]
		if t == nil || t.Mode != "live" || t.Status != StatusPending {
			return "payout " + ev.ID + " ignored (unknown or already final)"
		}
		if ev.Amount != 0 && ev.Amount != t.Amount {
			return "payout " + ev.ID + " AMOUNT MISMATCH, ignored"
		}
		a.finishTransferLocked(t, ev.Success, ev.Reason)
		return "payout " + ev.ID + " settled"
	}
	t := a.s.d.Txs[ev.ID]
	if t == nil || t.Mode != "live" || t.Method != code || t.Status != StatusPending {
		return "payment " + ev.ID + " ignored (unknown or already final)"
	}
	if ev.Amount != 0 && ev.Amount != t.Amount {
		return "payment " + ev.ID + " AMOUNT MISMATCH, ignored"
	}
	if ev.ProviderRef != "" {
		t.ProviderRef = ev.ProviderRef
	}
	if ev.Success {
		if err := a.succeedLocked(t); err != nil {
			return "payment " + ev.ID + " ledger error"
		}
		return "payment " + ev.ID + " succeeded"
	}
	reason := ev.Reason
	if reason == "" {
		reason = "payment was not completed"
	}
	a.failLocked(t, reason)
	return "payment " + ev.ID + " failed"
}

// livePayout sends a payout through the bank_payout adapter, outside the lock.
func (a *App) livePayout(id string, lp LiveProvider, c Creds, req LivePayout) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	res, err := lp.Payout(ctx, c, req)
	cancel()
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	t := a.s.d.Transfers[id]
	if t == nil || t.Status != StatusPending {
		return
	}
	if err != nil {
		var d *DeclineError
		if errors.As(err, &d) {
			a.finishTransferLocked(t, false, d.Reason)
			return
		}
		// Unclear outcome: the money may have left. Do NOT reverse; flag for the owner.
		a.auditLocked("system", "payout.needs_review", "transfer "+id+": "+clipStr(err.Error(), 200), "")
		return
	}
	if res.Done {
		a.finishTransferLocked(t, true, "")
	}
}

func clipStr(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
