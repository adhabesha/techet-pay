package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
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
			a.audit(r, "admin.login_failed", "email="+clipStr(strings.ToLower(strings.TrimSpace(in.Email)), 80))
			fail(w, err)
			return
		}
		a.audit(r, "admin.login", "")
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
		a.audit(r, "merchant.suspend", "merchant="+r.PathValue("id")+" suspended="+boolStr(in.Suspended))
		ok(w, "Updated", nil)
	}))
	mux.HandleFunc("POST /admin/api/merchants/{id}/live", a.adminAuth(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Enabled bool `json:"enabled"`
		}
		if !decode(w, r, &in) {
			return
		}
		if err := a.adminSetLive(r.PathValue("id"), in.Enabled); err != nil {
			fail(w, err)
			return
		}
		a.audit(r, "merchant.live_access", "merchant="+r.PathValue("id")+" enabled="+boolStr(in.Enabled))
		ok(w, "Updated", nil)
	}))
	mux.HandleFunc("GET /admin/api/providers", a.adminAuth(func(w http.ResponseWriter, r *http.Request) {
		ok(w, "ok", a.adminProviders(a.baseURL(r)))
	}))
	mux.HandleFunc("PUT /admin/api/providers/{code}", a.adminAuth(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Mode   string            `json:"mode"`
			Fields map[string]string `json:"fields"`
			Clear  []string          `json:"clear"`
		}
		if !decode(w, r, &in) {
			return
		}
		changed, err := a.adminSaveProvider(r.PathValue("code"), in.Mode, in.Fields, in.Clear)
		if err != nil {
			fail(w, err)
			return
		}
		a.audit(r, "provider.update", r.PathValue("code")+" mode="+in.Mode+" fields_changed=["+strings.Join(changed, ",")+"]")
		ok(w, "Saved", nil)
	}))
	mux.HandleFunc("POST /admin/api/providers/{code}/check", a.adminAuth(func(w http.ResponseWriter, r *http.Request) {
		msg, err := a.adminCheckProvider(r.PathValue("code"))
		a.audit(r, "provider.check", r.PathValue("code")+": "+clipStr(msg, 120))
		if err != nil {
			fail(w, err)
			return
		}
		ok(w, msg, nil)
	}))
	mux.HandleFunc("GET /admin/api/audit", a.adminAuth(func(w http.ResponseWriter, r *http.Request) {
		ok(w, "ok", a.adminAudit())
	}))
	mux.HandleFunc("GET /admin/api/backup", a.adminAuth(func(w http.ResponseWriter, r *http.Request) {
		b, err := a.backupJSON()
		if err != nil {
			fail(w, err)
			return
		}
		a.audit(r, "backup.download", "")
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="xidex-backup-`+time.Now().UTC().Format("2006-01-02-1504")+`.json"`)
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(b)
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
			"suspended": m.Suspended, "live": m.LiveEnabled, "webhook": m.WebhookURL != ""})
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

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func auditIP(r *http.Request) string {
	if f := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]); f != "" {
		return clipStr(f, 45)
	}
	h := r.RemoteAddr
	if i := strings.LastIndex(h, ":"); i > 0 {
		h = h[:i]
	}
	return clipStr(h, 45)
}

// ---- audit log ----

const auditMax = 5000

func (a *App) auditLocked(actor, action, detail, ip string) {
	a.s.d.Seq++
	a.s.d.Audit = append(a.s.d.Audit, AuditEntry{ID: a.s.d.Seq, At: time.Now().UTC(), Actor: actor, Action: action, Detail: clipStr(detail, 300), IP: ip})
	if len(a.s.d.Audit) > auditMax {
		a.s.d.Audit = append([]AuditEntry(nil), a.s.d.Audit[len(a.s.d.Audit)-auditMax:]...)
	}
	a.s.markDirty()
}

func (a *App) audit(r *http.Request, action, detail string) {
	actor := "admin"
	if strings.HasPrefix(action, "callback") {
		actor = "provider"
	}
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	a.auditLocked(actor, action, detail, auditIP(r))
}

func (a *App) adminAudit() []AuditEntry {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	n := len(a.s.d.Audit)
	from := 0
	if n > 300 {
		from = n - 300
	}
	out := make([]AuditEntry, 0, n-from)
	for i := n - 1; i >= from; i-- { // newest first
		out = append(out, a.s.d.Audit[i])
	}
	return out
}

// backupJSON exports everything except login sessions. Provider credentials stay
// encrypted inside it, so the file is useless to a thief without TECHET_SECRET_KEY,
// but it still holds merchant password hashes: keep it somewhere private.
func (a *App) backupJSON() ([]byte, error) {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	cp := a.s.d
	cp.Sessions = map[string]*Session{}
	return json.MarshalIndent(cp, "", " ")
}

func (a *App) adminSetLive(id string, v bool) error {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	m := a.s.d.Merchants[id]
	if m == nil {
		return notFound("merchant not found")
	}
	m.LiveEnabled = v
	a.s.markDirty()
	return nil
}

// ---- providers page ----

func (a *App) adminProviders(base string) []map[string]any {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	out := []map[string]any{}
	for _, lp := range liveProviderList() {
		pc := a.s.d.Providers[lp.Code()]
		fields := []map[string]any{}
		for _, f := range lp.Fields() {
			set := pc != nil && pc.Fields[f.Key] != ""
			fields = append(fields, map[string]any{"key": f.Key, "label": f.Label, "hint": f.Hint, "secret": f.Secret,
				"required": f.Required, "multiline": f.Multiline, "set": set})
		}
		v := map[string]any{"code": lp.Code(), "name": lp.Name(), "kind": lp.Kind(), "implemented": lp.Implemented(),
			"mode": "sandbox", "fields": fields, "callback_url": a.callbackURL(base, lp.Code()),
			"vault_ready": a.vault != nil, "missing": missingRequired(lp, pc)}
		if pc != nil {
			v["mode"] = pc.Mode
			v["updated_at"] = pc.UpdatedAt
			if !pc.LastCheckAt.IsZero() {
				v["last_check"] = map[string]any{"at": pc.LastCheckAt, "ok": pc.LastCheckOK, "message": pc.LastCheckMsg}
			}
		}
		out = append(out, v)
	}
	return out
}

func (a *App) adminSaveProvider(code, mode string, fields map[string]string, clear []string) ([]string, error) {
	lp := liveProviders[code]
	if lp == nil {
		return nil, notFound("unknown provider")
	}
	if mode != "sandbox" && mode != "live" {
		return nil, bad("mode must be sandbox or live")
	}
	allowed := map[string]bool{}
	for _, f := range lp.Fields() {
		allowed[f.Key] = true
	}
	for k, v := range fields {
		if !allowed[k] {
			return nil, bad("unknown field: " + k)
		}
		if len(v) > 8000 {
			return nil, bad("value too long for " + k)
		}
	}
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	pc := a.s.d.Providers[code]
	if pc == nil {
		pc = &ProviderConfig{Code: code, Mode: "sandbox", Fields: map[string]string{}}
	}
	var changed []string
	staged := map[string]string{}
	for k, v := range fields {
		v = strings.TrimSpace(v)
		if v == "" {
			continue // blank means "keep the stored value"
		}
		if a.vault == nil {
			return nil, bad("credentials cannot be stored yet: set TECHET_SECRET_KEY (16+ random characters) in the server environment first")
		}
		sealed, err := a.vault.Seal(v)
		if err != nil {
			return nil, err
		}
		staged[k] = sealed
		changed = append(changed, k)
	}
	next := map[string]string{}
	for k, v := range pc.Fields {
		next[k] = v
	}
	for k, v := range staged {
		next[k] = v
	}
	for _, k := range clear {
		if allowed[k] {
			delete(next, k)
			changed = append(changed, k+"(cleared)")
		}
	}
	probe := &ProviderConfig{Fields: next}
	if mode == "live" {
		if !lp.Implemented() {
			return nil, bad(lp.Name() + " cannot go live yet: its adapter (provider_" + code + ".go) has not been implemented")
		}
		if a.vault == nil {
			return nil, bad("set TECHET_SECRET_KEY on the server before going live")
		}
		if miss := missingRequired(lp, probe); len(miss) > 0 {
			return nil, bad("missing required fields: " + strings.Join(miss, ", "))
		}
		if !pc.LastCheckOK || pc.LastCheckAt.IsZero() || len(changed) > 0 {
			return nil, bad("run \"Test connection\" successfully with the saved credentials before switching to live")
		}
	}
	sort.Strings(changed)
	pc.Fields, pc.Mode, pc.UpdatedAt = next, mode, time.Now().UTC()
	if len(changed) > 0 { // credentials changed: they must be re-tested
		pc.LastCheckOK, pc.LastCheckMsg = false, "credentials changed: test again"
	}
	a.s.d.Providers[code] = pc
	a.s.markDirty()
	return changed, nil
}

func (a *App) adminCheckProvider(code string) (string, error) {
	lp := liveProviders[code]
	if lp == nil {
		return "", notFound("unknown provider")
	}
	a.s.mu.Lock()
	pc := a.s.d.Providers[code]
	if miss := missingRequired(lp, pc); len(miss) > 0 {
		a.s.mu.Unlock()
		return "missing required fields: " + strings.Join(miss, ", "), bad("missing required fields: " + strings.Join(miss, ", "))
	}
	creds, err := a.credsLocked(code)
	a.s.mu.Unlock()
	if err != nil {
		return err.Error(), bad(err.Error())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cerr := lp.Check(ctx, creds)
	msg := "Connection OK"
	if cerr != nil {
		msg = clipStr(cerr.Error(), 200)
	}
	a.s.mu.Lock()
	if pc = a.s.d.Providers[code]; pc != nil {
		pc.LastCheckAt, pc.LastCheckOK, pc.LastCheckMsg = time.Now().UTC(), cerr == nil, msg
		a.s.markDirty()
	}
	a.s.mu.Unlock()
	if cerr != nil {
		return msg, bad(msg)
	}
	return msg, nil
}
