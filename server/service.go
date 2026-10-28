package main

import (
	"crypto/pbkdf2"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/mail"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
)

type App struct {
	cfg      Config
	s        *Store
	prov     Provider
	lim      *limiter
	adm      *adminSessions
	vault    *Vault
	lastBase atomic.Value // last public address seen in a request (fallback for callback URLs)
}

type apiError struct {
	Code int
	Msg  string
}

func (e *apiError) Error() string { return e.Msg }
func bad(msg string) error        { return &apiError{400, msg} }
func notFound(msg string) error   { return &apiError{404, msg} }
func conflict(msg string) error   { return &apiError{409, msg} }

// ---- merchants & auth ----

func hashPassword(pw, saltHex string) string {
	salt, _ := hex.DecodeString(saltHex)
	k, _ := pbkdf2.Key(sha256.New, pw, salt, 210_000, 32)
	return hex.EncodeToString(k)
}

// Register creates a merchant. The secret key is returned once and only its
// hash is stored.
func (a *App) Register(name, email, password string) (Merchant, string, error) {
	name = strings.TrimSpace(name)
	addr, err := mail.ParseAddress(strings.TrimSpace(email))
	if err != nil || len(email) > 200 {
		return Merchant{}, "", bad("a valid email is required")
	}
	email = strings.ToLower(addr.Address)
	if len(name) < 2 || len(name) > 80 {
		return Merchant{}, "", bad("business name must be 2-80 characters")
	}
	if len(password) < 8 || len(password) > 200 {
		return Merchant{}, "", bad("password must be at least 8 characters")
	}
	salt := randHex(16)
	hash := hashPassword(password, salt) // slow: do before taking the lock

	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	if _, ok := a.s.emailIdx[email]; ok {
		return Merchant{}, "", conflict("an account with this email already exists")
	}
	secret := "TPSECK_TEST-" + randHex(20)
	m := &Merchant{
		ID: "mer_" + randHex(8), Name: name, Email: email, PassSalt: salt, PassHash: hash,
		PublicKey: "TPPUBK_TEST-" + randHex(12), SecretHash: sha(secret),
		SecretHint: secret[:16] + "…" + secret[len(secret)-4:], WebhookSecret: "whsec_" + randHex(16),
		CreatedAt: time.Now().UTC(),
	}
	a.s.d.Merchants[m.ID] = m
	a.s.keyIdx[m.SecretHash] = m.ID
	a.s.emailIdx[email] = m.ID
	a.s.markDirty()
	return *m, secret, nil
}

func (a *App) RotateKey(id string) (Merchant, string, error) {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	m := a.s.d.Merchants[id]
	if m == nil {
		return Merchant{}, "", notFound("merchant not found")
	}
	delete(a.s.keyIdx, m.SecretHash)
	secret := "TPSECK_TEST-" + randHex(20)
	m.SecretHash = sha(secret)
	m.SecretHint = secret[:16] + "…" + secret[len(secret)-4:]
	a.s.keyIdx[m.SecretHash] = m.ID
	a.s.markDirty()
	return *m, secret, nil
}

func (a *App) Login(email, password string) (string, Merchant, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	a.s.mu.Lock()
	id := a.s.emailIdx[email]
	var m Merchant
	if mp := a.s.d.Merchants[id]; mp != nil {
		m = *mp
	}
	a.s.mu.Unlock()

	// Always run the KDF so unknown emails take the same time as wrong passwords.
	salt := m.PassSalt
	if salt == "" {
		salt = "00000000000000000000000000000000"
	}
	got := hashPassword(password, salt)
	if m.ID == "" || subtle.ConstantTimeCompare([]byte(got), []byte(m.PassHash)) != 1 {
		return "", Merchant{}, &apiError{401, "invalid email or password"}
	}
	if m.Suspended {
		return "", Merchant{}, &apiError{403, "this account is suspended, contact Xidex support"}
	}
	tok := "sess_" + randHex(24)
	a.s.mu.Lock()
	a.s.d.Sessions[sha(tok)] = &Session{MerchantID: m.ID, Expires: time.Now().Add(12 * time.Hour)}
	a.s.markDirty()
	a.s.mu.Unlock()
	return tok, m, nil
}

func (a *App) Logout(tok string) {
	a.s.mu.Lock()
	delete(a.s.d.Sessions, sha(tok))
	a.s.markDirty()
	a.s.mu.Unlock()
}

func (a *App) merchantBySecret(key string) (Merchant, bool) {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	if id, ok := a.s.keyIdx[sha(key)]; ok && !a.s.d.Merchants[id].Suspended {
		return *a.s.d.Merchants[id], true
	}
	return Merchant{}, false
}

func (a *App) merchantBySession(tok string) (Merchant, bool) {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	h := sha(tok)
	se := a.s.d.Sessions[h]
	if se == nil || time.Now().After(se.Expires) {
		delete(a.s.d.Sessions, h)
		return Merchant{}, false
	}
	m := a.s.d.Merchants[se.MerchantID]
	if m == nil || m.Suspended {
		return Merchant{}, false
	}
	return *m, true
}

func (a *App) SetWebhook(id, rawURL string) error {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL != "" {
		if err := validateWebhookURL(rawURL); err != nil {
			return bad(err.Error())
		}
	}
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	a.s.d.Merchants[id].WebhookURL = rawURL
	a.s.markDirty()
	return nil
}

// ---- transactions ----

type InitReq struct {
	Amount      Money  `json:"amount"`
	Currency    string `json:"currency"`
	Email       string `json:"email"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	TxRef       string `json:"tx_ref"`
	ReturnURL   string `json:"return_url"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

var refRe = regexp.MustCompile(`^[A-Za-z0-9._~-]{1,100}$`)

func (a *App) fee(amount Money) Money {
	return Money((int64(amount)*a.cfg.FeeBPS + 5000) / 10000) // half-up
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n]
	}
	return s
}

func (a *App) InitTransaction(m Merchant, in InitReq) (*Transaction, error) {
	cur := strings.ToUpper(strings.TrimSpace(in.Currency))
	if cur == "" {
		cur = "ETB"
	}
	if cur != "ETB" {
		return nil, bad("currency must be ETB")
	}
	if in.Amount < MinAmount || in.Amount > MaxAmount {
		return nil, bad("amount must be between 1.00 and 1000000.00 ETB")
	}
	if !refRe.MatchString(in.TxRef) {
		return nil, bad("tx_ref is required (1-100 chars: letters, digits, . _ ~ -)")
	}
	if in.Email != "" {
		if _, err := mail.ParseAddress(in.Email); err != nil {
			return nil, bad("email is not valid")
		}
	}
	if in.ReturnURL != "" {
		if err := validateWebhookURL(in.ReturnURL); err != nil {
			return nil, bad("return_url must be an http(s) URL")
		}
	}
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	if _, dup := a.s.refIdx[m.ID+"|"+in.TxRef]; dup {
		return nil, conflict("tx_ref already used; each transaction needs a unique tx_ref")
	}
	now := time.Now().UTC()
	t := &Transaction{
		ID: "ch_" + randHex(10), CheckoutToken: randHex(20), MerchantID: m.ID, TxRef: in.TxRef,
		Amount: in.Amount, Fee: a.fee(in.Amount), Currency: cur, Email: clip(in.Email, 200),
		FirstName: clip(in.FirstName, 60), LastName: clip(in.LastName, 60), Title: clip(in.Title, 60),
		Description: clip(in.Description, 200), ReturnURL: strings.TrimSpace(in.ReturnURL),
		Status: StatusPending, CreatedAt: now, UpdatedAt: now, ExpiresAt: now.Add(a.cfg.TxTTL),
	}
	a.s.d.Txs[t.ID] = t
	a.s.refIdx[m.ID+"|"+t.TxRef] = t.ID
	a.s.tokIdx[t.CheckoutToken] = t.ID
	a.s.markDirty()
	cp := *t
	return &cp, nil
}

func (a *App) txByToken(tok string) (*Transaction, error) {
	id, ok := a.s.tokIdx[tok]
	if !ok {
		return nil, notFound("payment link not found")
	}
	t := a.s.d.Txs[id]
	a.expireLocked(t)
	return t, nil
}

func (a *App) expireLocked(t *Transaction) {
	if t.Status == StatusPending && time.Now().After(t.ExpiresAt) {
		a.failLocked(t, "payment session expired")
	}
}

func (a *App) CheckoutInfo(tok string) (map[string]any, error) {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	t, err := a.txByToken(tok)
	if err != nil {
		return nil, err
	}
	merchant := a.s.d.Merchants[t.MerchantID].Name
	return map[string]any{
		"merchant": merchant, "title": t.Title, "description": t.Description,
		"amount": t.Amount, "currency": t.Currency, "status": t.Status, "stage": t.Stage,
		"method": t.Method, "account": t.Account, "fail_reason": t.FailReason,
		"return_url": t.ReturnURL, "tx_ref": t.TxRef, "reference": t.ProviderRef,
		"methods": methods, "expires_at": t.ExpiresAt, "redirect_url": t.RedirectURL, "livemode": t.Mode == "live", "live": a.s.d.Merchants[t.MerchantID].LiveEnabled,
	}, nil
}

func (a *App) Pay(tok string, in PayInput) error { return a.PayAt(tok, in, a.cfg.BaseURL) }

func (a *App) PayAt(tok string, in PayInput, base string) error {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	t, err := a.txByToken(tok)
	if err != nil {
		return err
	}
	if t.Status != StatusPending {
		return conflict("this payment is already " + t.Status)
	}
	meth, ok := methodByCode(in.Method)
	if !ok {
		return bad("choose a payment method")
	}
	lp, creds, live, rerr := a.routeLocked(a.s.d.Merchants[t.MerchantID], meth.Code)
	if rerr != nil {
		return rerr
	}
	if live {
		return a.payLiveLocked(t, meth, in, lp, creds, base)
	}
	acct, needOTP, perr := a.prov.Charge(meth, in)
	if perr != nil && acct == "" { // input validation problem: let the payer retry
		return bad(perr.Error())
	}
	t.Method, t.Mode, t.Account, t.UpdatedAt = meth.Code, "sandbox", acct, time.Now().UTC()
	if perr != nil { // provider declined: terminal
		a.failLocked(t, perr.Error())
		return nil
	}
	if needOTP {
		t.Stage, t.OTPAttempts = "otp", 0
	}
	a.s.markDirty()
	return nil
}

func (a *App) ConfirmOTP(tok, otp string) error {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	t, err := a.txByToken(tok)
	if err != nil {
		return err
	}
	if t.Status != StatusPending || t.Stage != "otp" {
		return conflict("no confirmation is pending for this payment")
	}
	if t.Mode == "live" {
		return conflict("this payment is confirmed on the provider's side")
	}
	if !a.prov.ConfirmOTP(otp) {
		t.OTPAttempts++
		if t.OTPAttempts >= 3 {
			a.failLocked(t, "too many incorrect codes")
			return nil
		}
		a.s.markDirty()
		return bad("incorrect code, please try again")
	}
	return a.succeedLocked(t)
}

func (a *App) succeedLocked(t *Transaction) error {
	net := t.Amount - t.Fee
	err := a.s.postLocked(t.ID, "payment "+t.TxRef,
		Line{Account: acctClearing, Debit: t.Amount},
		Line{Account: acctMerchant(t.MerchantID), Credit: net},
		Line{Account: acctRevenue, Credit: t.Fee})
	if err != nil {
		return err
	}
	t.Status, t.Stage, t.UpdatedAt = StatusSuccess, "", time.Now().UTC()
	if t.Mode != "live" || t.ProviderRef == "" {
		t.ProviderRef = strings.ToUpper(t.Method[:2]) + randHex(5)
	}
	a.enqueueLocked(t.MerchantID, "charge.success", a.txView(t))
	a.s.markDirty()
	return nil
}

func (a *App) failLocked(t *Transaction, reason string) {
	t.Status, t.Stage, t.FailReason, t.UpdatedAt = StatusFailed, "", reason, time.Now().UTC()
	a.enqueueLocked(t.MerchantID, "charge.failed", a.txView(t))
	a.s.markDirty()
}

func (a *App) txView(t *Transaction) map[string]any {
	return map[string]any{
		"tx_ref": t.TxRef, "reference": t.ProviderRef, "status": t.Status, "amount": t.Amount,
		"fee": t.Fee, "net_amount": t.Amount - t.Fee, "currency": t.Currency, "email": t.Email,
		"first_name": t.FirstName, "last_name": t.LastName, "payment_method": t.Method,
		"account": t.Account, "failure_reason": t.FailReason, "created_at": t.CreatedAt,
		"updated_at": t.UpdatedAt, "livemode": t.Mode == "live",
	}
}

func (a *App) VerifyTransaction(m Merchant, ref string) (map[string]any, error) {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	id, ok := a.s.refIdx[m.ID+"|"+ref]
	if !ok {
		return nil, notFound("transaction not found")
	}
	t := a.s.d.Txs[id]
	a.expireLocked(t)
	return a.txView(t), nil
}

func (a *App) ListTransactions(m Merchant, status string, limit int) []map[string]any {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	var list []*Transaction
	for _, t := range a.s.d.Txs {
		if t.MerchantID != m.ID {
			continue
		}
		a.expireLocked(t)
		if status == "" || t.Status == status {
			list = append(list, t)
		}
	}
	sortTx(list)
	if limit > 0 && len(list) > limit {
		list = list[:limit]
	}
	out := make([]map[string]any, 0, len(list))
	for _, t := range list {
		v := a.txView(t)
		v["checkout_token"] = t.CheckoutToken
		out = append(out, v)
	}
	return out
}

// ---- balances & payouts ----

func (a *App) Balance(m Merchant) map[string]any {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	return map[string]any{"currency": "ETB", "available_balance": a.s.d.Balances[acctMerchant(m.ID)]}
}

type TransferReq struct {
	AccountName   string `json:"account_name"`
	AccountNumber string `json:"account_number"`
	Amount        Money  `json:"amount"`
	Reference     string `json:"reference"`
	BankCode      string `json:"bank_code"`
}

func (a *App) CreateTransfer(m Merchant, in TransferReq) (map[string]any, error) {
	bank, ok := bankByCode(in.BankCode)
	if !ok {
		return nil, bad("unknown bank_code (see GET /v1/banks)")
	}
	if !refRe.MatchString(in.Reference) {
		return nil, bad("reference is required (1-100 chars: letters, digits, . _ ~ -)")
	}
	if n := len(strings.TrimSpace(in.AccountName)); n < 2 || n > 80 {
		return nil, bad("account_name is required")
	}
	in.AccountNumber = strings.TrimSpace(in.AccountNumber)
	if err := a.prov.ValidateDestination(bank, in.AccountNumber); err != nil {
		return nil, bad(err.Error())
	}
	return a.createTransfer(m, in, bank)
}

func (a *App) createTransfer(m Merchant, in TransferReq, bank Bank) (map[string]any, error) {
	if in.Amount < MinAmount || in.Amount > MaxAmount {
		return nil, bad("amount must be between 1.00 and 1000000.00 ETB")
	}
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	if _, dup := a.s.xferIdx[m.ID+"|"+in.Reference]; dup {
		return nil, conflict("reference already used")
	}
	lp, pcreds, live, rerr := a.routeLocked(a.s.d.Merchants[m.ID], "bank_payout")
	if rerr != nil {
		return nil, bad("live payouts are not available yet")
	}
	total := in.Amount + a.cfg.PayoutFee
	if a.s.d.Balances[acctMerchant(m.ID)] < total {
		return nil, bad("insufficient balance: need " + total.String() + " ETB including the " + a.cfg.PayoutFee.String() + " ETB payout fee")
	}
	now := time.Now().UTC()
	t := &Transfer{ID: "tr_" + randHex(8), MerchantID: m.ID, Reference: in.Reference,
		AccountName: clip(in.AccountName, 80), AccountNumber: in.AccountNumber, BankCode: bank.Code,
		Amount: in.Amount, Fee: a.cfg.PayoutFee, Currency: "ETB", Mode: "sandbox", Status: StatusPending, CreatedAt: now, UpdatedAt: now}
	if live {
		t.Mode = "live"
	}
	// Hold funds immediately so the same balance can't be spent twice.
	if err := a.s.postLocked(t.ID, "payout hold "+t.Reference,
		Line{Account: acctMerchant(m.ID), Debit: total},
		Line{Account: acctPayoutHold, Credit: in.Amount},
		Line{Account: acctRevenue, Credit: t.Fee}); err != nil {
		return nil, err
	}
	a.s.d.Transfers[t.ID] = t
	a.s.xferIdx[m.ID+"|"+t.Reference] = t.ID
	a.s.markDirty()
	if live {
		go a.livePayout(t.ID, lp, pcreds, LivePayout{TransferID: t.ID, Reference: t.Reference, Amount: t.Amount,
			Currency: t.Currency, BankCode: t.BankCode, AccountName: t.AccountName, AccountNumber: t.AccountNumber,
			CallbackURL: a.callbackURL(a.cfg.BaseURL, "bank_payout")})
	} else {
		time.AfterFunc(a.cfg.PayoutDelay, func() { a.settleTransfer(t.ID) })
	}
	return a.transferView(t), nil
}

func (a *App) settleTransfer(id string) {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	t := a.s.d.Transfers[id]
	if t == nil || t.Status != StatusPending {
		return
	}
	ok, reason := a.prov.PayoutOutcome(t.AccountNumber)
	a.finishTransferLocked(t, ok, reason)
}

// finishTransferLocked settles a pending payout, sandbox or live. s.mu must be held.
func (a *App) finishTransferLocked(t *Transfer, ok bool, reason string) {
	t.UpdatedAt = time.Now().UTC()
	if ok {
		_ = a.s.postLocked(t.ID, "payout settled "+t.Reference,
			Line{Account: acctPayoutHold, Debit: t.Amount}, Line{Account: acctPayoutDone, Credit: t.Amount})
		t.Status = StatusSuccess
		a.enqueueLocked(t.MerchantID, "transfer.success", a.transferView(t))
	} else {
		// Reverse the hold and the fee: merchant is made whole.
		_ = a.s.postLocked(t.ID, "payout reversed "+t.Reference,
			Line{Account: acctPayoutHold, Debit: t.Amount}, Line{Account: acctRevenue, Debit: t.Fee},
			Line{Account: acctMerchant(t.MerchantID), Credit: t.Amount + t.Fee})
		t.Status, t.FailReason = StatusFailed, reason
		a.enqueueLocked(t.MerchantID, "transfer.failed", a.transferView(t))
	}
	a.s.markDirty()
}

func (a *App) transferView(t *Transfer) map[string]any {
	return map[string]any{"id": t.ID, "reference": t.Reference, "status": t.Status, "amount": t.Amount,
		"fee": t.Fee, "currency": t.Currency, "bank_code": t.BankCode, "account_name": t.AccountName,
		"account_number": t.AccountNumber, "failure_reason": t.FailReason, "created_at": t.CreatedAt, "livemode": t.Mode == "live"}
}

func (a *App) VerifyTransfer(m Merchant, ref string) (map[string]any, error) {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	id, ok := a.s.xferIdx[m.ID+"|"+ref]
	if !ok {
		return nil, notFound("transfer not found")
	}
	return a.transferView(a.s.d.Transfers[id]), nil
}

func (a *App) ListTransfers(m Merchant) []map[string]any {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	var list []*Transfer
	for _, t := range a.s.d.Transfers {
		if t.MerchantID == m.ID {
			list = append(list, t)
		}
	}
	sortTransfers(list)
	out := make([]map[string]any, 0, len(list))
	for _, t := range list {
		out = append(out, a.transferView(t))
	}
	return out
}

func (a *App) Overview(m Merchant) map[string]any {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	var vol, net Money
	var ok, failed, pending int
	days := map[string]Money{}
	for _, t := range a.s.d.Txs {
		if t.MerchantID != m.ID {
			continue
		}
		a.expireLocked(t)
		switch t.Status {
		case StatusSuccess:
			ok++
			vol += t.Amount
			net += t.Amount - t.Fee
			days[t.UpdatedAt.Format("2006-01-02")] += t.Amount
		case StatusFailed:
			failed++
		default:
			pending++
		}
	}
	series := []map[string]any{}
	for i := 13; i >= 0; i-- {
		d := time.Now().UTC().AddDate(0, 0, -i).Format("2006-01-02")
		series = append(series, map[string]any{"date": d, "volume": days[d]})
	}
	return map[string]any{
		"available_balance": a.s.d.Balances[acctMerchant(m.ID)], "gross_volume": vol, "net_earned": net,
		"successful": ok, "failed": failed, "pending": pending, "series": series,
		"fee_percent": float64(a.cfg.FeeBPS) / 100, "payout_fee": a.cfg.PayoutFee,
	}
}

// ---- webhooks ----

func (a *App) enqueueLocked(merchantID, event string, payload map[string]any) {
	m := a.s.d.Merchants[merchantID]
	if m == nil || m.WebhookURL == "" {
		return
	}
	body, _ := json.Marshal(map[string]any{"event": event, "data": payload})
	d := &Delivery{ID: "whd_" + randHex(8), MerchantID: merchantID, Event: event, URL: m.WebhookURL,
		Secret: m.WebhookSecret, Payload: string(body), Status: "pending",
		NextAttempt: time.Now(), CreatedAt: time.Now().UTC()}
	a.s.d.Deliveries[d.ID] = d
}
