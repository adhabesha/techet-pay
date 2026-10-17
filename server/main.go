package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

//go:embed web/*
var webFS embed.FS

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func defaultConfig() Config {
	bps, _ := strconv.ParseInt(env("TECHET_FEE_BPS", "250"), 10, 64)
	pf, err := parseMoney(env("TECHET_PAYOUT_FEE", "5.00"))
	if err != nil {
		pf = 500
	}
	return Config{
		BaseURL:              strings.TrimRight(env("TECHET_BASE_URL", os.Getenv("RENDER_EXTERNAL_URL")), "/"),
		DataPath:             env("TECHET_DATA", "techet-data.json"),
		FeeBPS:               bps,
		PayoutFee:            pf,
		AllowPrivateWebhooks: os.Getenv("TECHET_ALLOW_PRIVATE_WEBHOOKS") == "1",
		AdminEmail:           strings.ToLower(env("TECHET_ADMIN_EMAIL", "admin@techet.pay")),
		AdminPassword:        os.Getenv("TECHET_ADMIN_PASSWORD"),
		PayoutDelay:          3 * time.Second,
		TxTTL:                30 * time.Minute,
		Backoff:              []time.Duration{5 * time.Second, 30 * time.Second, 2 * time.Minute, 10 * time.Minute, time.Hour},
	}
}

func NewApp(cfg Config) (*App, error) {
	s, err := newStore(cfg.DataPath, newRemote(os.Getenv("UPSTASH_REDIS_REST_URL"), os.Getenv("UPSTASH_REDIS_REST_TOKEN")))
	if err != nil {
		return nil, err
	}
	return &App{cfg: cfg, s: s, prov: SandboxProvider{}, lim: newLimiter(), adm: newAdminSessions()}, nil
}

func listenAddr() string {
	if a := os.Getenv("TECHET_ADDR"); a != "" {
		return a
	}
	if p := os.Getenv("PORT"); p != "" { // Render, Fly, Heroku-style hosts
		return ":" + p
	}
	return ":8080"
}

func main() {
	cfg := defaultConfig()
	app, err := NewApp(cfg)
	if err != nil {
		log.Fatal(err)
	}
	stop := make(chan struct{})
	go app.RunWebhooks(stop)
	srv := &http.Server{Addr: listenAddr(), Handler: app.Routes(), ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	go func() {
		log.Printf("TechET Pay (sandbox) listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	<-ctx.Done()
	log.Print("shutting down: saving data")
	sctx, c2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer c2()
	_ = srv.Shutdown(sctx)
	close(stop)
	app.s.Flush()
}

// baseURL is the public address used in checkout links. It comes from
// TECHET_BASE_URL (or Render's RENDER_EXTERNAL_URL), else from the request.
func (a *App) baseURL(r *http.Request) string {
	if a.cfg.BaseURL != "" {
		return a.cfg.BaseURL
	}
	proto := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0])
	if proto != "http" && proto != "https" {
		proto = "http"
		if r.TLS != nil {
			proto = "https"
		}
	}
	host := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Host"), ",")[0])
	if host == "" || strings.ContainsAny(host, "/ \\") {
		host = r.Host
	}
	return proto + "://" + host
}

// ---- rate limiting (fixed window, per client IP + bucket) ----

type limiter struct {
	mu sync.Mutex
	m  map[string]*win
}
type win struct {
	n     int
	reset time.Time
}

func newLimiter() *limiter { return &limiter{m: map[string]*win{}} }

func (l *limiter) allow(key string, max int, per time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if len(l.m) > 10000 {
		for k, w := range l.m {
			if now.After(w.reset) {
				delete(l.m, k)
			}
		}
	}
	w := l.m[key]
	if w == nil || now.After(w.reset) {
		l.m[key] = &win{1, now.Add(per)}
		return true
	}
	w.n++
	return w.n <= max
}

func clientIP(r *http.Request) string {
	h, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return h
}

// ---- JSON helpers ----

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func ok(w http.ResponseWriter, msg string, data any) {
	writeJSON(w, 200, map[string]any{"message": msg, "status": "success", "data": data})
}

func fail(w http.ResponseWriter, err error) {
	var ae *apiError
	if errors.As(err, &ae) {
		writeJSON(w, ae.Code, map[string]any{"message": ae.Msg, "status": "failed", "data": nil})
		return
	}
	log.Printf("internal error: %v", err)
	writeJSON(w, 500, map[string]any{"message": "internal error", "status": "failed", "data": nil})
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		fail(w, bad("invalid JSON body: "+err.Error()))
		return false
	}
	return true
}

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

type authed func(w http.ResponseWriter, r *http.Request, m Merchant)

func (a *App) limit(bucket string, max int, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.lim.allow(bucket+"|"+clientIP(r), max, time.Minute) {
			w.Header().Set("Retry-After", "60")
			fail(w, &apiError{429, "too many requests, slow down"})
			return
		}
		next(w, r)
	}
}

func (a *App) apiAuth(next authed) http.HandlerFunc {
	return a.limit("api", 600, func(w http.ResponseWriter, r *http.Request) {
		m, found := a.merchantBySecret(bearer(r))
		if !found {
			fail(w, &apiError{401, "invalid or missing secret key (send 'Authorization: Bearer <secret key>')"})
			return
		}
		next(w, r, m)
	})
}

func (a *App) dashAuth(next authed) http.HandlerFunc {
	return a.limit("dash", 600, func(w http.ResponseWriter, r *http.Request) {
		m, found := a.merchantBySession(bearer(r))
		if !found {
			fail(w, &apiError{401, "session expired, please sign in again"})
			return
		}
		next(w, r, m)
	})
}

func secure(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; img-src 'self' data:; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func page(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b, err := webFS.ReadFile("web/" + name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(b)
	}
}

func (a *App) Routes() http.Handler {
	mux := http.NewServeMux()

	// ---- public merchant API (Bearer secret key) ----
	mux.HandleFunc("POST /v1/transaction/initialize", a.apiAuth(func(w http.ResponseWriter, r *http.Request, m Merchant) {
		var in InitReq
		if !decode(w, r, &in) {
			return
		}
		t, err := a.InitTransaction(m, in)
		if err != nil {
			fail(w, err)
			return
		}
		ok(w, "Hosted Link", map[string]any{"checkout_url": a.baseURL(r) + "/checkout/" + t.CheckoutToken, "tx_ref": t.TxRef})
	}))
	mux.HandleFunc("GET /v1/transaction/verify/{ref}", a.apiAuth(func(w http.ResponseWriter, r *http.Request, m Merchant) {
		v, err := a.VerifyTransaction(m, r.PathValue("ref"))
		if err != nil {
			fail(w, err)
			return
		}
		ok(w, "Payment details fetched successfully", v)
	}))
	mux.HandleFunc("GET /v1/transactions", a.apiAuth(func(w http.ResponseWriter, r *http.Request, m Merchant) {
		lim, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if lim <= 0 || lim > 200 {
			lim = 50
		}
		ok(w, "Transactions retrieved", a.ListTransactions(m, r.URL.Query().Get("status"), lim))
	}))
	mux.HandleFunc("GET /v1/balance", a.apiAuth(func(w http.ResponseWriter, r *http.Request, m Merchant) {
		ok(w, "Balance retrieved", a.Balance(m))
	}))
	mux.HandleFunc("POST /v1/transfers", a.apiAuth(func(w http.ResponseWriter, r *http.Request, m Merchant) {
		var in TransferReq
		if !decode(w, r, &in) {
			return
		}
		v, err := a.CreateTransfer(m, in)
		if err != nil {
			fail(w, err)
			return
		}
		ok(w, "Transfer queued successfully", v)
	}))
	mux.HandleFunc("GET /v1/transfers/verify/{ref}", a.apiAuth(func(w http.ResponseWriter, r *http.Request, m Merchant) {
		v, err := a.VerifyTransfer(m, r.PathValue("ref"))
		if err != nil {
			fail(w, err)
			return
		}
		ok(w, "Transfer details fetched", v)
	}))
	mux.HandleFunc("GET /v1/banks", func(w http.ResponseWriter, r *http.Request) { ok(w, "Banks retrieved", banks) })
	mux.HandleFunc("GET /v1/payment-methods", func(w http.ResponseWriter, r *http.Request) {
		ok(w, "Payment methods retrieved", methods)
	})

	// ---- hosted checkout (capability = unguessable token in the URL) ----
	mux.HandleFunc("GET /checkout/{token}", page("checkout.html"))
	mux.HandleFunc("GET /checkout/{token}/info", a.limit("co-info", 120, func(w http.ResponseWriter, r *http.Request) {
		v, err := a.CheckoutInfo(r.PathValue("token"))
		if err != nil {
			fail(w, err)
			return
		}
		ok(w, "ok", v)
	}))
	mux.HandleFunc("POST /checkout/{token}/pay", a.limit("co-pay", 30, func(w http.ResponseWriter, r *http.Request) {
		var in PayInput
		if !decode(w, r, &in) {
			return
		}
		if err := a.Pay(r.PathValue("token"), in); err != nil {
			fail(w, err)
			return
		}
		ok(w, "ok", nil)
	}))
	mux.HandleFunc("POST /checkout/{token}/confirm", a.limit("co-otp", 30, func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			OTP string `json:"otp"`
		}
		if !decode(w, r, &in) {
			return
		}
		if err := a.ConfirmOTP(r.PathValue("token"), in.OTP); err != nil {
			fail(w, err)
			return
		}
		ok(w, "ok", nil)
	}))

	// ---- dashboard API (Bearer session token) ----
	mux.HandleFunc("POST /dash/register", a.limit("register", 10, func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Name, Email, Password string }
		if !decode(w, r, &in) {
			return
		}
		m, secret, err := a.Register(in.Name, in.Email, in.Password)
		if err != nil {
			fail(w, err)
			return
		}
		tok, _, err := a.Login(in.Email, in.Password)
		if err != nil {
			fail(w, err)
			return
		}
		ok(w, "Account created", map[string]any{"token": tok, "merchant": merchantView(m), "secret_key": secret})
	}))
	mux.HandleFunc("POST /dash/login", a.limit("login", 10, func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Email, Password string }
		if !decode(w, r, &in) {
			return
		}
		tok, m, err := a.Login(in.Email, in.Password)
		if err != nil {
			fail(w, err)
			return
		}
		ok(w, "Signed in", map[string]any{"token": tok, "merchant": merchantView(m)})
	}))
	mux.HandleFunc("POST /dash/logout", a.dashAuth(func(w http.ResponseWriter, r *http.Request, m Merchant) {
		a.Logout(bearer(r))
		ok(w, "Signed out", nil)
	}))
	mux.HandleFunc("GET /dash/me", a.dashAuth(func(w http.ResponseWriter, r *http.Request, m Merchant) {
		ok(w, "ok", merchantView(m))
	}))
	mux.HandleFunc("GET /dash/overview", a.dashAuth(func(w http.ResponseWriter, r *http.Request, m Merchant) {
		ok(w, "ok", a.Overview(m))
	}))
	mux.HandleFunc("GET /dash/transactions", a.dashAuth(func(w http.ResponseWriter, r *http.Request, m Merchant) {
		ok(w, "ok", a.ListTransactions(m, r.URL.Query().Get("status"), 100))
	}))
	mux.HandleFunc("GET /dash/transfers", a.dashAuth(func(w http.ResponseWriter, r *http.Request, m Merchant) {
		ok(w, "ok", a.ListTransfers(m))
	}))
	mux.HandleFunc("POST /dash/transfers", a.dashAuth(func(w http.ResponseWriter, r *http.Request, m Merchant) {
		var in TransferReq
		if !decode(w, r, &in) {
			return
		}
		if in.Reference == "" {
			in.Reference = "payout-" + randHex(6)
		}
		v, err := a.CreateTransfer(m, in)
		if err != nil {
			fail(w, err)
			return
		}
		ok(w, "Transfer queued successfully", v)
	}))
	mux.HandleFunc("GET /dash/deliveries", a.dashAuth(func(w http.ResponseWriter, r *http.Request, m Merchant) {
		ok(w, "ok", a.ListDeliveries(m.ID))
	}))
	mux.HandleFunc("POST /dash/webhook", a.dashAuth(func(w http.ResponseWriter, r *http.Request, m Merchant) {
		var in struct {
			URL string `json:"url"`
		}
		if !decode(w, r, &in) {
			return
		}
		if err := a.SetWebhook(m.ID, in.URL); err != nil {
			fail(w, err)
			return
		}
		ok(w, "Webhook saved", nil)
	}))
	mux.HandleFunc("POST /dash/rotate-key", a.dashAuth(func(w http.ResponseWriter, r *http.Request, m Merchant) {
		nm, secret, err := a.RotateKey(m.ID)
		if err != nil {
			fail(w, err)
			return
		}
		ok(w, "Secret key rotated; the old key no longer works", map[string]any{"merchant": merchantView(nm), "secret_key": secret})
	}))
	mux.HandleFunc("POST /dash/test-payment", a.dashAuth(func(w http.ResponseWriter, r *http.Request, m Merchant) {
		var in struct {
			Amount Money  `json:"amount"`
			Email  string `json:"email"`
		}
		if !decode(w, r, &in) {
			return
		}
		t, err := a.InitTransaction(m, InitReq{Amount: in.Amount, Email: in.Email, TxRef: "test-" + randHex(6),
			Title: "Test payment", Description: "Created from your TechET Pay dashboard"})
		if err != nil {
			fail(w, err)
			return
		}
		ok(w, "Test payment created", map[string]any{"checkout_url": a.baseURL(r) + "/checkout/" + t.CheckoutToken})
	}))
	mux.HandleFunc("GET /dash/ledger-check", a.dashAuth(func(w http.ResponseWriter, r *http.Request, m Merchant) {
		ok(w, "ok", a.LedgerCheck())
	}))

	a.adminRoutes(mux)

	mux.HandleFunc("GET /assets/{file}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("file")
		b, err := webFS.ReadFile("web/assets/" + name)
		if err != nil || strings.Contains(name, "..") {
			http.NotFound(w, r)
			return
		}
		if ct := mime.TypeByExtension(name[strings.LastIndex(name, "."):]); ct != "" {
			w.Header().Set("Content-Type", ct)
		}
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_, _ = w.Write(b)
	})
	mux.HandleFunc("GET /dashboard", page("dashboard.html"))
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/dashboard", http.StatusFound) })
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) })

	return secure(mux)
}

func merchantView(m Merchant) map[string]any {
	return map[string]any{"id": m.ID, "name": m.Name, "email": m.Email, "public_key": m.PublicKey,
		"secret_key_hint": m.SecretHint, "webhook_url": m.WebhookURL, "webhook_secret": m.WebhookSecret}
}

// LedgerCheck proves the books balance: debits == credits across all entries.
func (a *App) LedgerCheck() map[string]any {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	var dr, cr, sum Money
	for _, e := range a.s.d.Ledger {
		dr += e.Debit
		cr += e.Credit
	}
	for _, b := range a.s.d.Balances {
		sum += b
	}
	return map[string]any{"entries": len(a.s.d.Ledger), "total_debits": dr, "total_credits": cr, "balanced": dr == cr && sum == 0}
}
