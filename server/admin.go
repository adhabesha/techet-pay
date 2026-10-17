package main

import (
	"crypto/subtle"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// Admin = the gateway owner. One account, configured by environment variables
// (TECHET_ADMIN_EMAIL / TECHET_ADMIN_PASSWORD). If no password is set the admin
// panel is switched off.
type adminSessions struct {
	mu sync.Mutex
	m  map[string]time.Time // sha(token) -> expiry (in memory: a restart signs admins out)
}

func newAdminSessions() *adminSessions { return &adminSessions{m: map[string]time.Time{}} }

func (a *App) adminAuth(next http.HandlerFunc) http.HandlerFunc {
	return a.limit("admin", 600, func(w http.ResponseWriter, r *http.Request) {
		h := sha(bearer(r))
		a.adm.mu.Lock()
		exp, found := a.adm.m[h]
		if found && time.Now().After(exp) {
			delete(a.adm.m, h)
			found = false
		}
		a.adm.mu.Unlock()
		if !found {
			fail(w, &apiError{401, "admin session expired, please sign in again"})
			return
		}
		next(w, r)
	})
}

func (a *App) adminLogin(email, password string) (string, error) {
	if a.cfg.AdminPassword == "" {
		return "", &apiError{503, "admin panel is disabled: set TECHET_ADMIN_PASSWORD on the server"}
	}
	okE := subtle.ConstantTimeCompare([]byte(strings.ToLower(strings.TrimSpace(email))), []byte(a.cfg.AdminEmail))
	okP := subtle.ConstantTimeCompare([]byte(sha(password)), []byte(sha(a.cfg.AdminPassword)))
	if okE&okP != 1 {
		return "", &apiError{401, "invalid admin email or password"}
	}
	tok := "adm_" + randHex(24)
	a.adm.mu.Lock()
	a.adm.m[sha(tok)] = time.Now().Add(8 * time.Hour)
	a.adm.mu.Unlock()
	return tok, nil
}

func (a *App) adminRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin", page("admin.html"))
	mux.HandleFunc("POST /admin/api/login", a.limit("admin-login", 8, func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Email, Password string }
		if !decode(w, r, &in) {
			return
		}
		tok, err := a.adminLogin(in.Email, in.Password)
		if err != nil {
			fail(w, err)
			return
		}
		ok(w, "Signed in", map[string]any{"token": tok, "email": a.cfg.AdminEmail})
	}))
	mux.HandleFunc("POST /admin/api/logout", a.adminAuth(func(w http.ResponseWriter, r *http.Request) {
		a.adm.mu.Lock()
		delete(a.adm.m, sha(bearer(r)))
		a.adm.mu.Unlock()
		ok(w, "Signed out", nil)
	}))
	mux.HandleFunc("GET /admin/api/overview", a.adminAuth(func(w http.ResponseWriter, r *http.Request) {
		ok(w, "ok", a.adminOverview())
	}))
	mux.HandleFunc("GET /admin/api/merchants", a.adminAuth(func(w http.ResponseWriter, r *http.Request) {
		ok(w, "ok", a.adminMerchants())
	}))
	mux.HandleFunc("POST /admin/api/merchants/{id}/suspend", a.adminAuth(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Suspended bool `json:"suspended"`
		}
		if !decode(w, r, &in) {
			return
		}
		if err := a.adminSuspend(r.PathValue("id"), in.Suspended); err != nil {
			fail(w, err)
			return
		}
		ok(w, "Updated", nil)
	}))
	mux.HandleFunc("GET /admin/api/transactions", a.adminAuth(func(w http.ResponseWriter, r *http.Request) {
		ok(w, "ok", a.adminTxs(r.URL.Query().Get("status"), strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))))
	}))
	mux.HandleFunc("GET /admin/api/transfers", a.adminAuth(func(w http.ResponseWriter, r *http.Request) {
		ok(w, "ok", a.adminTransfers())
	}))
	mux.HandleFunc("GET /admin/api/deliveries", a.adminAuth(func(w http.ResponseWriter, r *http.Request) {
		ok(w, "ok", a.adminDeliveries())
	}))
	mux.HandleFunc("GET /admin/api/ledger-check", a.adminAuth(func(w http.ResponseWriter, r *http.Request) {
		ok(w, "ok", a.LedgerCheck())
	}))
}

func (a *App) merchantName(id string) string {
	if m := a.s.d.Merchants[id]; m != nil {
		return m.Name
	}
	return id
}

func (a *App) adminOverview() map[string]any {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	var vol, fees, held Money
	var okN, failed, pending int
	days := map[string]Money{}
	byMethod := map[string]Money{}
	for _, t := range a.s.d.Txs {
		a.expireLocked(t)
		switch t.Status {
		case StatusSuccess:
			okN++
			vol += t.Amount
			fees += t.Fee
			days[t.UpdatedAt.Format("2006-01-02")] += t.Amount
			byMethod[t.Method] += t.Amount
		case StatusFailed:
			failed++
		default:
			pending++
		}
	}
	for id := range a.s.d.Merchants {
		held += a.s.d.Balances[acctMerchant(id)]
	}
	var pOK, pFail, pPend int
	var paid Money
	for _, t := range a.s.d.Transfers {
		switch t.Status {
		case StatusSuccess:
			pOK++
			paid += t.Amount
		case StatusFailed:
			pFail++
		default:
			pPend++
		}
	}
	susp := 0
	for _, m := range a.s.d.Merchants {
		if m.Suspended {
			susp++
		}
	}
	series := []map[string]any{}
	for i := 13; i >= 0; i-- {
		d := time.Now().UTC().AddDate(0, 0, -i).Format("2006-01-02")
		series = append(series, map[string]any{"date": d, "volume": days[d]})
	}
	methods := []map[string]any{}
	for k, v := range byMethod {
		methods = append(methods, map[string]any{"method": k, "volume": v})
	}
	sort.Slice(methods, func(i, j int) bool { return methods[i]["volume"].(Money) > methods[j]["volume"].(Money) })
	return map[string]any{
		"merchants": len(a.s.d.Merchants), "suspended": susp, "gross_volume": vol, "fees_earned": a.s.d.Balances[acctRevenue],
		"fees_from_payments": fees, "merchant_balances": held, "paid_out": paid,
		"successful": okN, "failed": failed, "pending": pending,
		"payouts_ok": pOK, "payouts_failed": pFail, "payouts_pending": pPend,
		"series": series, "methods": methods, "fee_percent": float64(a.cfg.FeeBPS) / 100, "payout_fee": a.cfg.PayoutFee,
	}
}

func (a *App) adminMerchants() []map[string]any {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	vol := map[string]Money{}
	cnt := map[string]int{}
	for _, t := range a.s.d.Txs {
		if t.Status == StatusSuccess {
			vol[t.MerchantID] += t.Amount
			cnt[t.MerchantID]++
		}
	}
	list := []*Merchant{}
	for _, m := range a.s.d.Merchants {
		list = append(list, m)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].CreatedAt.After(list[j].CreatedAt) })
	out := []map[string]any{}
	for _, m := range list {
		out = append(out, map[string]any{"id": m.ID, "name": m.Name, "email": m.Email, "created_at": m.CreatedAt,
			"volume": vol[m.ID], "payments": cnt[m.ID], "balance": a.s.d.Balances[acctMerchant(m.ID)],
			"suspended": m.Suspended, "webhook": m.WebhookURL != ""})
	}
	return out
}

func (a *App) adminSuspend(id string, v bool) error {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	m := a.s.d.Merchants[id]
	if m == nil {
		return notFound("merchant not found")
	}
	m.Suspended = v
	if v { // kick active dashboard sessions
		for h, se := range a.s.d.Sessions {
			if se.MerchantID == id {
				delete(a.s.d.Sessions, h)
			}
		}
	}
	a.s.markDirty()
	return nil
}

func (a *App) adminTxs(status, q string) []map[string]any {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	var list []*Transaction
	for _, t := range a.s.d.Txs {
		a.expireLocked(t)
		if status != "" && t.Status != status {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(t.TxRef+" "+t.Email+" "+t.ProviderRef+" "+a.merchantName(t.MerchantID)), q) {
			continue
		}
		list = append(list, t)
	}
	sortTx(list)
	if len(list) > 200 {
		list = list[:200]
	}
	out := []map[string]any{}
	for _, t := range list {
		v := a.txView(t)
		v["merchant"] = a.merchantName(t.MerchantID)
		out = append(out, v)
	}
	return out
}

func (a *App) adminTransfers() []map[string]any {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	var list []*Transfer
	for _, t := range a.s.d.Transfers {
		list = append(list, t)
	}
	sortTransfers(list)
	if len(list) > 200 {
		list = list[:200]
	}
	out := []map[string]any{}
	for _, t := range list {
		v := a.transferView(t)
		v["merchant"] = a.merchantName(t.MerchantID)
		out = append(out, v)
	}
	return out
}

func (a *App) adminDeliveries() []map[string]any {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	var l []*Delivery
	for _, d := range a.s.d.Deliveries {
		l = append(l, d)
	}
	sort.Slice(l, func(i, j int) bool { return l[i].CreatedAt.After(l[j].CreatedAt) })
	if len(l) > 100 {
		l = l[:100]
	}
	out := []map[string]any{}
	for _, d := range l {
		out = append(out, map[string]any{"id": d.ID, "merchant": a.merchantName(d.MerchantID), "event": d.Event,
			"status": d.Status, "attempts": d.Attempts, "last_code": d.LastCode, "last_error": d.LastError, "created_at": d.CreatedAt})
	}
	return out
}
