package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeLive is a test-only adapter standing in for a real provider.
type fakeLive struct {
	stubAdapter
	declineNext bool
	charges     []LiveCharge
	payouts     []LivePayout
}

func (f *fakeLive) Implemented() bool                  { return true }
func (f *fakeLive) Check(context.Context, Creds) error { return nil }
func (f *fakeLive) Charge(_ context.Context, c Creds, r LiveCharge) (LiveChargeResult, error) {
	if c["api_key"] != "secret-value" {
		return LiveChargeResult{}, errors.New("bad creds")
	}
	f.charges = append(f.charges, r)
	if f.declineNext {
		return LiveChargeResult{}, &DeclineError{"insufficient funds"}
	}
	return LiveChargeResult{Account: "0911****44", ProviderRef: "PRV123", RedirectURL: "https://provider.example/pay/1"}, nil
}
func (f *fakeLive) ParseCallback(c Creds, r *http.Request) (CallbackEvent, error) {
	if r.Header.Get("X-Sig") != c["api_key"] { // stands in for real signature verification
		return CallbackEvent{}, errors.New("bad signature")
	}
	var in struct {
		Kind, ID, Status string
		Amount           Money
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	return CallbackEvent{Kind: in.Kind, ID: in.ID, Success: in.Status == "ok", Amount: in.Amount}, nil
}
func (f *fakeLive) Payout(_ context.Context, _ Creds, r LivePayout) (LivePayoutResult, error) {
	f.payouts = append(f.payouts, r)
	return LivePayoutResult{ProviderRef: "PO1", Done: true}, nil
}

func newFake(code string, kind string) *fakeLive {
	return &fakeLive{stubAdapter: stubAdapter{code: code, name: code, kind: kind, fields: []CredField{
		{Key: "api_key", Label: "API key", Secret: true, Required: true},
		{Key: "short_code", Label: "Short code", Required: true},
	}}}
}

func TestVaultRoundTrip(t *testing.T) {
	if _, err := newVault("short"); err == nil {
		t.Fatal("short key must be rejected")
	}
	v, _ := newVault("a-long-enough-secret-key")
	s1, _ := v.Seal("hello")
	s2, _ := v.Seal("hello")
	if s1 == s2 || strings.Contains(s1, "hello") {
		t.Fatal("sealed values must be randomised and not contain the plaintext")
	}
	if p, err := v.Open(s1); err != nil || p != "hello" {
		t.Fatalf("open: %q %v", p, err)
	}
	v2, _ := newVault("another-long-secret-key!")
	if _, err := v2.Open(s1); err == nil {
		t.Fatal("wrong key must not decrypt")
	}
}

func TestProvidersAndLiveRouting(t *testing.T) {
	// swap in working fake adapters, restore afterwards
	oldT, oldB := liveProviders["telebirr"], liveProviders["bank_payout"]
	ft, fb := newFake("telebirr", "payment"), newFake("bank_payout", "payout")
	liveProviders["telebirr"], liveProviders["bank_payout"] = ft, fb
	defer func() { liveProviders["telebirr"], liveProviders["bank_payout"] = oldT, oldB }()

	cfg := defaultConfig()
	cfg.DataPath, cfg.AdminPassword, cfg.SecretKey = "", "correct-horse-battery", "a-long-enough-secret-key"
	cfg.PayoutDelay = 50 * time.Millisecond
	app, err := NewApp(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(app.Routes())
	defer ts.Close()
	app.cfg.BaseURL = ts.URL
	e := &env2{t, app, ts}

	_, lg := e.call("POST", "/admin/api/login", "", map[string]string{"email": cfg.AdminEmail, "password": "correct-horse-battery"})
	at := data2(lg)["token"].(string)

	// provider list: nothing secret, stubs not implemented, sandbox by default
	_, pl := e.call("GET", "/admin/api/providers", at, nil)
	list := pl["data"].([]any)
	if len(list) != 6 {
		t.Fatalf("want 6 providers, got %d", len(list))
	}
	if c, _ := e.call("GET", "/admin/api/providers", "", nil); c != 401 {
		t.Fatal("providers need admin")
	}
	// a stub adapter can never go live
	if c, r := e.call("PUT", "/admin/api/providers/mpesa", at, map[string]any{"mode": "live"}); c != 400 || !strings.Contains(r["message"].(string), "not been implemented") {
		t.Fatalf("stub must refuse live: %d %v", c, r)
	}
	// check on a stub fails honestly
	e.call("PUT", "/admin/api/providers/mpesa", at, map[string]any{"mode": "sandbox", "fields": map[string]string{"consumer_key": "k", "consumer_secret": "s", "short_code": "1", "passkey": "p", "base_url": "https://x"}})
	if c, _ := e.call("POST", "/admin/api/providers/mpesa/check", at, nil); c != 400 {
		t.Fatalf("check on stub must fail, got %d", c)
	}
	// unknown field / bad mode rejected
	if c, _ := e.call("PUT", "/admin/api/providers/telebirr", at, map[string]any{"mode": "sandbox", "fields": map[string]string{"nope": "x"}}); c != 400 {
		t.Fatalf("unknown field: %d", c)
	}
	if c, _ := e.call("PUT", "/admin/api/providers/telebirr", at, map[string]any{"mode": "maybe"}); c != 400 {
		t.Fatalf("bad mode: %d", c)
	}
	// going live needs complete credentials AND a successful test
	if c, _ := e.call("PUT", "/admin/api/providers/telebirr", at, map[string]any{"mode": "live"}); c != 400 {
		t.Fatalf("live without credentials: %d", c)
	}
	e.call("PUT", "/admin/api/providers/telebirr", at, map[string]any{"mode": "sandbox", "fields": map[string]string{"api_key": "secret-value", "short_code": "5555"}})
	if c, _ := e.call("PUT", "/admin/api/providers/telebirr", at, map[string]any{"mode": "live"}); c != 400 {
		t.Fatalf("live before a successful test must be refused: %d", c)
	}
	if c, _ := e.call("POST", "/admin/api/providers/telebirr/check", at, nil); c != 200 {
		t.Fatalf("check: %d", c)
	}
	// credentials never leave the server and are encrypted at rest
	_, pl = e.call("GET", "/admin/api/providers", at, nil)
	raw, _ := json.Marshal(pl)
	if strings.Contains(string(raw), "secret-value") || strings.Contains(string(raw), "5555") {
		t.Fatal("credential values leaked through the admin API")
	}
	if sealed := app.s.d.Providers["telebirr"].Fields["api_key"]; sealed == "" || strings.Contains(sealed, "secret-value") {
		t.Fatalf("credential not encrypted at rest: %q", sealed)
	}
	bk, _ := json.Marshal(app.s.d)
	if strings.Contains(string(bk), "secret-value") {
		t.Fatal("plaintext credential in snapshot")
	}

	// merchants: one sandbox, one live
	reg := func(name, mail string) (string, string, string) {
		_, r := e.call("POST", "/dash/register", "", map[string]string{"name": name, "email": mail, "password": "password123"})
		d := data2(r)
		return d["token"].(string), d["secret_key"].(string), d["merchant"].(map[string]any)["id"].(string)
	}
	_, skSand, _ := reg("Sandbox Shop", "s@x.co")
	_, skLive, liveID := reg("Live Shop", "l@x.co")
	pay := func(sk, ref string) string {
		_, r := e.call("POST", "/v1/transaction/initialize", sk, map[string]any{"amount": "200.00", "email": "p@x.co", "tx_ref": ref})
		u := data2(r)["checkout_url"].(string)
		return u[len(ts.URL+"/checkout/"):]
	}

	// provider still sandbox: live merchant is NOT silently downgraded
	e.call("POST", "/admin/api/merchants/"+liveID+"/live", at, map[string]bool{"enabled": true})
	tokL := pay(skLive, "L1")
	if c, r := e.call("POST", "/checkout/"+tokL+"/pay", "", map[string]string{"method": "telebirr", "phone": "0911223344"}); c != 400 || !strings.Contains(r["message"].(string), "not available") {
		t.Fatalf("live merchant + sandbox provider must be refused: %d %v", c, r)
	}
	// switch the provider live
	if c, r := e.call("PUT", "/admin/api/providers/telebirr", at, map[string]any{"mode": "live"}); c != 200 {
		t.Fatalf("go live: %d %v", c, r)
	}
	// sandbox merchant still gets the sandbox (OTP flow), even though the provider is live
	tokS := pay(skSand, "S1")
	if c, _ := e.call("POST", "/checkout/"+tokS+"/pay", "", map[string]string{"method": "telebirr", "phone": "0911223344"}); c != 200 {
		t.Fatalf("sandbox pay: %d", c)
	}
	if c, _ := e.call("POST", "/checkout/"+tokS+"/confirm", "", map[string]string{"otp": "123456"}); c != 200 {
		t.Fatalf("sandbox otp: %d", c)
	}
	if len(ft.charges) != 0 {
		t.Fatal("sandbox merchant must never reach the live adapter")
	}
	// live merchant: redirect flow, no OTP, no card data to us
	if c, r := e.call("POST", "/checkout/"+tokL+"/pay", "", map[string]string{"method": "telebirr", "phone": "0911223344"}); c != 200 {
		t.Fatalf("live pay: %d %v", c, r)
	}
	if len(ft.charges) != 1 || ft.charges[0].CallbackURL != ts.URL+"/callbacks/telebirr" || ft.charges[0].Amount != 20000 {
		t.Fatalf("adapter got wrong request: %+v", ft.charges)
	}
	_, info := e.call("GET", "/checkout/"+tokL+"/info", "", nil)
	if d := data2(info); d["stage"] != "redirect" || d["redirect_url"] != "https://provider.example/pay/1" || d["livemode"] != true {
		t.Fatalf("checkout info: %v", d)
	}
	if c, _ := e.call("POST", "/checkout/"+tokL+"/confirm", "", map[string]string{"otp": "123456"}); c != 409 {
		t.Fatalf("OTP must not work on live payments: %d", c)
	}
	if c, _ := e.call("POST", "/checkout/"+tokL+"/pay", "", map[string]string{"method": "telebirr", "phone": "0911223344"}); c != 409 {
		t.Fatalf("double start must be refused: %d", c)
	}
	txID := ft.charges[0].TxID

	callback := func(sig string, body map[string]any) int {
		b, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", ts.URL+"/callbacks/telebirr", strings.NewReader(string(b)))
		req.Header.Set("X-Sig", sig)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if c := callback("forged", map[string]any{"id": txID, "status": "ok"}); c != 400 {
		t.Fatalf("forged callback must be rejected: %d", c)
	}
	if c := callback("secret-value", map[string]any{"id": txID, "status": "ok", "amount": "1.00"}); c != 200 {
		t.Fatal("callback status")
	}
	if app.s.d.Txs[txID].Status != StatusPending {
		t.Fatal("amount mismatch must not settle the payment")
	}
	if c := callback("secret-value", map[string]any{"id": txID, "status": "ok", "amount": "200.00"}); c != 200 {
		t.Fatal("callback status")
	}
	if tx := app.s.d.Txs[txID]; tx.Status != StatusSuccess || tx.ProviderRef != "PRV123" || tx.Mode != "live" {
		t.Fatalf("live payment not settled: %+v", tx)
	}
	callback("secret-value", map[string]any{"id": txID, "status": "fail"}) // replay after final: ignored
	if app.s.d.Txs[txID].Status != StatusSuccess {
		t.Fatal("a final payment must not change")
	}
	if bal := app.s.d.Balances[acctMerchant(liveID)]; bal != 19500 {
		t.Fatalf("live merchant balance: %v", bal)
	}
	if ck := app.LedgerCheck(); ck["balanced"] != true {
		t.Fatalf("ledger: %v", ck)
	}

	// provider decline is terminal
	ft.declineNext = true
	tok2 := pay(skLive, "L2")
	e.call("POST", "/checkout/"+tok2+"/pay", "", map[string]string{"method": "telebirr", "phone": "0911223344"})
	if tx := app.s.d.Txs[app.s.tokIdx[tok2]]; tx.Status != StatusFailed || tx.FailReason != "insufficient funds" {
		t.Fatalf("decline: %+v", tx)
	}

	// payouts: live merchant needs the bank_payout provider live
	body := map[string]any{"account_name": "Abebe", "account_number": "1000123456789", "amount": "50.00", "reference": "po-1", "bank_code": "cbe"}
	if c, r := e.call("POST", "/v1/transfers", skLive, body); c != 400 {
		t.Fatalf("live payout before provider live: %d %v", c, r)
	}
	e.call("PUT", "/admin/api/providers/bank_payout", at, map[string]any{"mode": "sandbox", "fields": map[string]string{"api_key": "secret-value", "short_code": "1"}})
	e.call("POST", "/admin/api/providers/bank_payout/check", at, nil)
	if c, r := e.call("PUT", "/admin/api/providers/bank_payout", at, map[string]any{"mode": "live"}); c != 200 {
		t.Fatalf("payout live: %d %v", c, r)
	}
	if c, r := e.call("POST", "/v1/transfers", skLive, body); c != 200 {
		t.Fatalf("live payout: %d %v", c, r)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		app.s.mu.Lock()
		var st string
		for _, tr := range app.s.d.Transfers {
			st = tr.Status
		}
		app.s.mu.Unlock()
		if st == StatusSuccess {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(fb.payouts) != 1 {
		t.Fatalf("payout adapter calls: %d", len(fb.payouts))
	}

	// audit log + backup
	_, au := e.call("GET", "/admin/api/audit", at, nil)
	ar, _ := json.Marshal(au)
	for _, want := range []string{"admin.login", "provider.update", "provider.check", "merchant.live_access", "callback.rejected", "callback.accepted"} {
		if !strings.Contains(string(ar), want) {
			t.Fatalf("audit log is missing %q", want)
		}
	}
	if strings.Contains(string(ar), "secret-value") {
		t.Fatal("credential in audit log")
	}
	req, _ := http.NewRequest("GET", ts.URL+"/admin/api/backup", nil)
	req.Header.Set("Authorization", "Bearer "+at)
	resp, _ := http.DefaultClient.Do(req)
	rb, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(rb), `"merchants"`) || strings.Contains(string(rb), "secret-value") || !strings.Contains(resp.Header.Get("Content-Disposition"), "xidex-backup") {
		t.Fatalf("backup: %d", resp.StatusCode)
	}
}

func TestProviderCredentialsNeedSecretKey(t *testing.T) {
	cfg := defaultConfig()
	cfg.DataPath, cfg.AdminPassword = "", "correct-horse-battery"
	app, _ := NewApp(cfg) // no TECHET_SECRET_KEY
	ts := httptest.NewServer(app.Routes())
	defer ts.Close()
	e := &env2{t, app, ts}
	_, lg := e.call("POST", "/admin/api/login", "", map[string]string{"email": cfg.AdminEmail, "password": "correct-horse-battery"})
	at := data2(lg)["token"].(string)
	if c, r := e.call("PUT", "/admin/api/providers/telebirr", at, map[string]any{"mode": "sandbox", "fields": map[string]string{"app_id": "x"}}); c != 400 || !strings.Contains(r["message"].(string), "TECHET_SECRET_KEY") {
		t.Fatalf("must refuse to store credentials without a key: %d %v", c, r)
	}
}
