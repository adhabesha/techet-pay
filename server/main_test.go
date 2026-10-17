package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type env2 struct {
	t   *testing.T
	app *App
	ts  *httptest.Server
}

func (e *env2) call(method, path, bearerTok string, body any) (int, map[string]any) {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, e.ts.URL+path, rd)
	if bearerTok != "" {
		req.Header.Set("Authorization", "Bearer "+bearerTok)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func data2(m map[string]any) map[string]any { d, _ := m["data"].(map[string]any); return d }

func TestEndToEnd(t *testing.T) {
	cfg := defaultConfig()
	cfg.DataPath = ""
	cfg.AllowPrivateWebhooks = true
	cfg.PayoutDelay = 150 * time.Millisecond
	cfg.Backoff = []time.Duration{200 * time.Millisecond}
	app, err := NewApp(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(app.Routes())
	defer ts.Close()
	app.cfg.BaseURL = ts.URL
	stop := make(chan struct{})
	defer close(stop)
	go app.RunWebhooks(stop)
	e := &env2{t, app, ts}

	// webhook receiver: fail the first delivery to prove retries, then accept.
	var mu sync.Mutex
	var got []map[string]any
	var sigOK bool
	var hits int
	var secret string
	rcv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		defer mu.Unlock()
		hits++
		if hits == 1 {
			w.WriteHeader(500)
			return
		}
		tsn, _ := strconv.ParseInt(r.Header.Get("X-TechET-Timestamp"), 10, 64)
		sigOK = r.Header.Get("X-TechET-Signature") == sign(secret, tsn, body)
		var m map[string]any
		_ = json.Unmarshal(body, &m)
		got = append(got, m)
		w.WriteHeader(200)
	}))
	defer rcv.Close()

	// register + weak input rejected
	if c, _ := e.call("POST", "/dash/register", "", map[string]string{"name": "Shop", "email": "a@b.co", "password": "short"}); c != 400 {
		t.Fatalf("weak password should be 400, got %d", c)
	}
	c, r := e.call("POST", "/dash/register", "", map[string]string{"name": "Addis Shop", "email": "owner@addis.shop", "password": "correct-horse-battery"})
	if c != 200 {
		t.Fatalf("register: %d %v", c, r)
	}
	d := data2(r)
	sess, sk := d["token"].(string), d["secret_key"].(string)
	secret = d["merchant"].(map[string]any)["webhook_secret"].(string)
	if c, _ := e.call("POST", "/dash/register", "", map[string]string{"name": "Dup", "email": "OWNER@addis.shop", "password": "correct-horse-battery"}); c != 409 {
		t.Fatalf("duplicate email should be 409, got %d", c)
	}
	if c, _ := e.call("POST", "/dash/webhook", sess, map[string]string{"url": rcv.URL}); c != 200 {
		t.Fatal("set webhook failed")
	}

	// auth required
	if c, _ := e.call("POST", "/v1/transaction/initialize", "bogus", map[string]any{"amount": "10"}); c != 401 {
		t.Fatalf("bad key should be 401, got %d", c)
	}
	// validation
	if c, _ := e.call("POST", "/v1/transaction/initialize", sk, map[string]any{"amount": "0.50", "tx_ref": "x1"}); c != 400 {
		t.Fatalf("amount below minimum should be 400, got %d", c)
	}

	// initialize (amount as number AND dup tx_ref)
	c, r = e.call("POST", "/v1/transaction/initialize", sk, map[string]any{"amount": 1000, "currency": "ETB", "email": "buyer@x.com", "tx_ref": "order-1"})
	if c != 200 {
		t.Fatalf("init: %d %v", c, r)
	}
	url := data2(r)["checkout_url"].(string)
	tok := url[len(ts.URL+"/checkout/"):]
	if c, _ := e.call("POST", "/v1/transaction/initialize", sk, map[string]any{"amount": "10", "tx_ref": "order-1"}); c != 409 {
		t.Fatalf("duplicate tx_ref should be 409, got %d", c)
	}

	// verify while pending
	_, r = e.call("GET", "/v1/transaction/verify/order-1", sk, nil)
	if data2(r)["status"] != "pending" {
		t.Fatalf("expected pending, got %v", r)
	}

	// pay: invalid phone -> retryable; wrong OTP; right OTP
	if c, _ := e.call("POST", "/checkout/"+tok+"/pay", "", map[string]string{"method": "telebirr", "phone": "123"}); c != 400 {
		t.Fatalf("invalid phone should be 400, got %d", c)
	}
	if c, r := e.call("POST", "/checkout/"+tok+"/pay", "", map[string]string{"method": "telebirr", "phone": "+251 911 223 344"}); c != 200 {
		t.Fatalf("pay: %d %v", c, r)
	}
	if c, _ := e.call("POST", "/checkout/"+tok+"/confirm", "", map[string]string{"otp": "000000"}); c != 400 {
		t.Fatal("wrong OTP should be 400")
	}
	if c, r := e.call("POST", "/checkout/"+tok+"/confirm", "", map[string]string{"otp": "123456"}); c != 200 {
		t.Fatalf("confirm: %d %v", c, r)
	}
	// can't pay twice
	if c, _ := e.call("POST", "/checkout/"+tok+"/pay", "", map[string]string{"method": "telebirr", "phone": "0911223344"}); c != 409 {
		t.Fatalf("double pay should be 409, got %d", c)
	}

	_, r = e.call("GET", "/v1/transaction/verify/order-1", sk, nil)
	v := data2(r)
	if v["status"] != "success" || v["fee"] != "25.00" || v["net_amount"] != "975.00" {
		t.Fatalf("unexpected tx after payment: %v", v)
	}
	_, r = e.call("GET", "/v1/balance", sk, nil)
	if data2(r)["available_balance"] != "975.00" {
		t.Fatalf("balance: %v", r)
	}

	// a declined card is terminal and credits nothing
	_, r = e.call("POST", "/v1/transaction/initialize", sk, map[string]any{"amount": "200", "tx_ref": "order-2"})
	tok2 := data2(r)["checkout_url"].(string)[len(ts.URL+"/checkout/"):]
	e.call("POST", "/checkout/"+tok2+"/pay", "", map[string]string{"method": "card", "card_number": "4000 0000 0000 0002", "expiry": "12/39", "cvc": "123"})
	_, r = e.call("GET", "/v1/transaction/verify/order-2", sk, nil)
	if data2(r)["status"] != "failed" {
		t.Fatalf("declined card should fail: %v", r)
	}

	// card success path
	_, r = e.call("POST", "/v1/transaction/initialize", sk, map[string]any{"amount": "300", "tx_ref": "order-3"})
	tok3 := data2(r)["checkout_url"].(string)[len(ts.URL+"/checkout/"):]
	e.call("POST", "/checkout/"+tok3+"/pay", "", map[string]string{"method": "card", "card_number": "4242424242424242", "expiry": "12/39", "cvc": "123"})
	e.call("POST", "/checkout/"+tok3+"/confirm", "", map[string]string{"otp": "123456"})
	_, r = e.call("GET", "/v1/balance", sk, nil)
	if data2(r)["available_balance"] != "1267.50" { // 975 + 292.50
		t.Fatalf("balance after 2nd payment: %v", r)
	}

	// payouts: insufficient, success, rejected+refunded, duplicate reference
	if c, _ := e.call("POST", "/v1/transfers", sk, map[string]any{"account_name": "A", "account_number": "1000123456789", "amount": "5000", "reference": "p0", "bank_code": "cbe"}); c != 400 {
		t.Fatal("overdraw should be 400")
	}
	if c, r := e.call("POST", "/v1/transfers", sk, map[string]any{"account_name": "Abebe K", "account_number": "1000123456789", "amount": "500", "reference": "p1", "bank_code": "cbe"}); c != 200 {
		t.Fatalf("transfer: %d %v", c, r)
	}
	if c, _ := e.call("POST", "/v1/transfers", sk, map[string]any{"account_name": "Abebe K", "account_number": "1000123456789", "amount": "500", "reference": "p1", "bank_code": "cbe"}); c != 409 {
		t.Fatal("duplicate payout reference should be 409")
	}
	_, r = e.call("GET", "/v1/balance", sk, nil)
	if data2(r)["available_balance"] != "762.50" { // 1267.50 - 500 - 5 fee
		t.Fatalf("balance after payout hold: %v", r)
	}
	e.call("POST", "/v1/transfers", sk, map[string]any{"account_name": "Bad Acct", "account_number": "1000123450000", "amount": "100", "reference": "p2", "bank_code": "cbe"})
	time.Sleep(600 * time.Millisecond)
	_, r = e.call("GET", "/v1/transfers/verify/p1", sk, nil)
	if data2(r)["status"] != "success" {
		t.Fatalf("p1 should settle: %v", r)
	}
	_, r = e.call("GET", "/v1/transfers/verify/p2", sk, nil)
	if data2(r)["status"] != "failed" {
		t.Fatalf("p2 should fail: %v", r)
	}
	_, r = e.call("GET", "/v1/balance", sk, nil)
	if data2(r)["available_balance"] != "762.50" { // rejected payout fully refunded
		t.Fatalf("balance after refund: %v", r)
	}

	// webhooks: first attempt 500, retried, signed correctly
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(got)
		mu.Unlock()
		if n >= 4 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	mu.Lock()
	if len(got) < 4 || !sigOK {
		t.Fatalf("webhooks: got %d (want >=4) sigOK=%v", len(got), sigOK)
	}
	mu.Unlock()

	// the books must balance and the key rotates
	if lc := app.LedgerCheck(); lc["balanced"] != true {
		t.Fatalf("ledger unbalanced: %v", lc)
	}
	_, r = e.call("POST", "/dash/rotate-key", sess, map[string]string{})
	if c, _ := e.call("GET", "/v1/balance", sk, nil); c != 401 {
		t.Fatal("old key must stop working after rotation")
	}
	nk := data2(r)["secret_key"].(string)
	if c, _ := e.call("GET", "/v1/balance", nk, nil); c != 200 {
		t.Fatal("new key must work")
	}
}

func TestMoneyParsing(t *testing.T) {
	good := map[string]Money{"1": 100, "1.5": 150, "0.07": 7, "250.00": 25000}
	for s, want := range good {
		if got, err := parseMoney(s); err != nil || got != want {
			t.Errorf("%q => %v %v", s, got, err)
		}
	}
	for _, s := range []string{"", "-5", "1e3", "1.234", "abc", "1.2.3", "9999999999"} {
		if _, err := parseMoney(s); err == nil {
			t.Errorf("%q should be rejected", s)
		}
	}
}

// A fake Upstash-style REST endpoint: POST ["SET",k,v] / ["GET",k].
func fakeUpstash(t *testing.T, down *bool) (*httptest.Server, map[string]string) {
	store := map[string]string{}
	var mu sync.Mutex
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if *down {
			w.WriteHeader(503)
			return
		}
		if r.Header.Get("Authorization") != "Bearer tok" {
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}
		var cmd []string
		_ = json.NewDecoder(r.Body).Decode(&cmd)
		mu.Lock()
		defer mu.Unlock()
		switch cmd[0] {
		case "SET":
			store[cmd[1]] = cmd[2]
			_, _ = w.Write([]byte(`{"result":"OK"}`))
		case "GET":
			if v, ok := store[cmd[1]]; ok {
				b, _ := json.Marshal(map[string]string{"result": v})
				_, _ = w.Write(b)
			} else {
				_, _ = w.Write([]byte(`{"result":null}`))
			}
		}
	})), store
}

func TestRemoteSnapshotSurvivesRestart(t *testing.T) {
	down := false
	srv, kv := fakeUpstash(t, &down)
	defer srv.Close()
	cfg := defaultConfig()
	cfg.DataPath = ""

	s1, err := newStore("", newRemote(srv.URL, "tok"))
	if err != nil {
		t.Fatal(err)
	}
	a1 := &App{cfg: cfg, s: s1, prov: SandboxProvider{}, lim: newLimiter()}
	m1, _, err := a1.Register("Shop", "x@y.co", "long-enough-pw")
	if err != nil {
		t.Fatal(err)
	}
	// a real payment makes the clearing account negative: the snapshot must still reload
	tx, err := a1.InitTransaction(m1, InitReq{Amount: 100000, TxRef: "r1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := a1.Pay(tx.CheckoutToken, PayInput{Method: "telebirr", Phone: "0911223344"}); err != nil {
		t.Fatal(err)
	}
	if err := a1.ConfirmOTP(tx.CheckoutToken, "123456"); err != nil {
		t.Fatal(err)
	}
	s1.Flush() // what the SIGTERM handler does
	if len(kv[remoteKey]) == 0 {
		t.Fatal("snapshot was not saved remotely")
	}

	// "restart": a brand-new store must see the merchant.
	s2, err := newStore("", newRemote(srv.URL, "tok"))
	if err != nil {
		t.Fatal(err)
	}
	if len(s2.d.Merchants) != 1 {
		t.Fatalf("expected 1 merchant after restore, got %d", len(s2.d.Merchants))
	}
	a2 := &App{cfg: cfg, s: s2, prov: SandboxProvider{}, lim: newLimiter()}
	if got := a2.Balance(m1)["available_balance"]; got != Money(97500) {
		t.Fatalf("balance after restore = %v, want 975.00", got)
	}
	if lc := a2.LedgerCheck(); lc["balanced"] != true {
		t.Fatalf("restored ledger unbalanced: %v", lc)
	}

	// fail closed: an unreachable remote must stop startup instead of starting empty.
	down = true
	if testing.Short() {
		return
	}
	done := make(chan error, 1)
	go func() { _, err := newStore("", newRemote(srv.URL, "tok")); done <- err }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("startup must fail when the remote store is down")
		}
	case <-time.After(30 * time.Second):
		t.Fatal("startup did not give up")
	}
}

func TestBaseURLFromForwardedHeaders(t *testing.T) {
	a := &App{cfg: Config{}}
	r, _ := http.NewRequest("GET", "http://internal:10000/x", nil)
	r.Header.Set("X-Forwarded-Proto", "https")
	r.Header.Set("X-Forwarded-Host", "techet-pay.onrender.com")
	if got := a.baseURL(r); got != "https://techet-pay.onrender.com" {
		t.Fatalf("got %s", got)
	}
	a.cfg.BaseURL = "https://pay.example.com"
	if got := a.baseURL(r); got != "https://pay.example.com" {
		t.Fatalf("explicit base URL must win, got %s", got)
	}
}

func TestNegativeMoneyRoundTrip(t *testing.T) {
	b, _ := json.Marshal(map[string]Money{"a": -150025})
	var out map[string]Money
	if err := json.Unmarshal(b, &out); err != nil || out["a"] != -150025 {
		t.Fatalf("round trip failed: %s %v %v", b, out, err)
	}
}

func TestDiskSnapshotRestart(t *testing.T) {
	path := t.TempDir() + "/d.json"
	cfg := defaultConfig()
	s1, _ := newStore(path, nil)
	a1 := &App{cfg: cfg, s: s1, prov: SandboxProvider{}, lim: newLimiter()}
	m, _, _ := a1.Register("Shop", "d@y.co", "long-enough-pw")
	tx, _ := a1.InitTransaction(m, InitReq{Amount: 50000, TxRef: "r1"})
	_ = a1.Pay(tx.CheckoutToken, PayInput{Method: "telebirr", Phone: "0911223344"})
	_ = a1.ConfirmOTP(tx.CheckoutToken, "123456")
	s1.Flush()
	s2, err := newStore(path, nil)
	if err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	a2 := &App{cfg: cfg, s: s2, prov: SandboxProvider{}, lim: newLimiter()}
	if got := a2.Balance(m)["available_balance"]; got != Money(48750) {
		t.Fatalf("balance after reload = %v", got)
	}
}

func TestAdminPanel(t *testing.T) {
	cfg := defaultConfig()
	cfg.DataPath = ""
	cfg.AdminPassword = ""
	app, err := NewApp(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(app.Routes())
	defer ts.Close()
	e := &env2{t, app, ts}

	if c, _ := e.call("POST", "/admin/api/login", "", map[string]string{"email": cfg.AdminEmail, "password": "x"}); c != 503 {
		t.Fatalf("admin must be disabled without a password, got %d", c)
	}
	app.cfg.AdminPassword = "correct-horse-battery"
	if c, _ := e.call("POST", "/admin/api/login", "", map[string]string{"email": cfg.AdminEmail, "password": "nope"}); c != 401 {
		t.Fatalf("wrong password: want 401 got %d", c)
	}
	if c, _ := e.call("GET", "/admin/api/overview", "", nil); c != 401 {
		t.Fatalf("overview without session: want 401 got %d", c)
	}
	// a merchant session must not open the admin API
	_, reg := e.call("POST", "/dash/register", "", map[string]string{"name": "Shop", "email": "shop@example.com", "password": "password123"})
	mt, secret := data2(reg)["token"].(string), data2(reg)["secret_key"].(string)
	if c, _ := e.call("GET", "/admin/api/merchants", mt, nil); c != 401 {
		t.Fatalf("merchant token must not work on admin API, got %d", c)
	}
	c, lg := e.call("POST", "/admin/api/login", "", map[string]string{"email": cfg.AdminEmail, "password": "correct-horse-battery"})
	if c != 200 {
		t.Fatalf("admin login failed: %d", c)
	}
	at := data2(lg)["token"].(string)
	_, ov := e.call("GET", "/admin/api/overview", at, nil)
	if data2(ov)["merchants"].(float64) != 1 {
		t.Fatalf("overview merchants: %v", ov)
	}
	_, ms := e.call("GET", "/admin/api/merchants", at, nil)
	id := ms["data"].([]any)[0].(map[string]any)["id"].(string)

	if c, _ := e.call("POST", "/v1/transaction/initialize", secret, map[string]any{"amount": "100.00", "email": "a@b.co", "tx_ref": "adm-1"}); c != 200 {
		t.Fatalf("active merchant init: %d", c)
	}
	if c, _ := e.call("POST", "/admin/api/merchants/"+id+"/suspend", at, map[string]bool{"suspended": true}); c != 200 {
		t.Fatalf("suspend: %d", c)
	}
	if c, _ := e.call("POST", "/v1/transaction/initialize", secret, map[string]any{"amount": "100.00", "email": "a@b.co", "tx_ref": "adm-2"}); c != 401 {
		t.Fatalf("suspended key must be rejected, got %d", c)
	}
	if c, _ := e.call("GET", "/dash/me", mt, nil); c != 401 {
		t.Fatalf("suspended session must be rejected, got %d", c)
	}
	if c, _ := e.call("POST", "/dash/login", "", map[string]string{"email": "shop@example.com", "password": "password123"}); c != 403 {
		t.Fatalf("suspended login: want 403 got %d", c)
	}
	e.call("POST", "/admin/api/merchants/"+id+"/suspend", at, map[string]bool{"suspended": false})
	if c, _ := e.call("POST", "/v1/transaction/initialize", secret, map[string]any{"amount": "100.00", "email": "a@b.co", "tx_ref": "adm-3"}); c != 200 {
		t.Fatalf("reactivated merchant init: %d", c)
	}
	for _, p := range []string{"/admin/api/transactions", "/admin/api/transfers", "/admin/api/deliveries", "/admin/api/ledger-check"} {
		if c, _ := e.call("GET", p, at, nil); c != 200 {
			t.Fatalf("%s: %d", p, c)
		}
	}
}

func TestInstallableAppFiles(t *testing.T) {
	cfg := defaultConfig()
	cfg.DataPath = ""
	app, err := NewApp(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(app.Routes())
	defer ts.Close()
	for path, ct := range map[string]string{
		"/manifest-dashboard.webmanifest": "application/manifest+json", "/manifest-admin.webmanifest": "application/manifest+json",
		"/sw.js": "text/javascript", "/assets/icon-192.png": "image/png", "/assets/icon-512.png": "image/png", "/assets/icon-maskable-512.png": "image/png",
	} {
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), ct) {
			t.Fatalf("%s: status %d type %q", path, resp.StatusCode, resp.Header.Get("Content-Type"))
		}
	}
}
