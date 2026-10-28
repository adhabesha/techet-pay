package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"os"
	"sync"
	"time"
)

type data struct {
	Merchants  map[string]*Merchant       `json:"merchants"`
	Txs        map[string]*Transaction    `json:"txs"`
	Ledger     []LedgerEntry              `json:"ledger"`
	Balances   map[string]Money           `json:"balances"` // credit-normal: credits - debits
	Transfers  map[string]*Transfer       `json:"transfers"`
	Deliveries map[string]*Delivery       `json:"deliveries"`
	Sessions   map[string]*Session        `json:"sessions"` // key = sha256(token)
	Seq        int64                      `json:"seq"`
	Providers  map[string]*ProviderConfig `json:"providers"`
	Audit      []AuditEntry               `json:"audit"`
}

// Store is an in-memory database guarded by one mutex, snapshotted atomically
// to disk after every change. All exported-style methods on App lock s.mu.
type Store struct {
	mu    sync.Mutex
	path  string
	d     data
	dirty chan struct{}

	remote  *Remote
	flushMu sync.Mutex

	keyIdx   map[string]string // secret key hash -> merchant id
	emailIdx map[string]string // lower email -> merchant id
	refIdx   map[string]string // merchantID|tx_ref -> tx id
	tokIdx   map[string]string // checkout token -> tx id
	xferIdx  map[string]string // merchantID|reference -> transfer id
}

func newStore(path string, remote *Remote) (*Store, error) {
	s := &Store{path: path, remote: remote, dirty: make(chan struct{}, 1)}
	s.d = data{
		Merchants: map[string]*Merchant{}, Txs: map[string]*Transaction{},
		Balances: map[string]Money{}, Transfers: map[string]*Transfer{},
		Deliveries: map[string]*Delivery{}, Sessions: map[string]*Session{},
		Providers: map[string]*ProviderConfig{},
	}
	loaded := false
	if remote != nil {
		// Fail closed: if the remote store is unreachable we must NOT start empty,
		// or the first flush would overwrite the real data with an empty snapshot.
		var b []byte
		var err error
		for i := 0; i < 4; i++ {
			if b, err = remote.Load(); err == nil {
				break
			}
			time.Sleep(time.Duration(i+1) * 2 * time.Second)
		}
		if err != nil {
			return nil, err
		}
		if len(b) > 0 {
			if err := json.Unmarshal(b, &s.d); err != nil {
				return nil, err
			}
			loaded = true
		}
	}
	if !loaded && path != "" {
		if b, err := os.ReadFile(path); err == nil {
			if err := json.Unmarshal(b, &s.d); err != nil {
				return nil, err
			}
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}
	if s.d.Providers == nil {
		s.d.Providers = map[string]*ProviderConfig{}
	}
	s.reindex()
	if path != "" || remote != nil {
		go s.flusher()
	}
	return s, nil
}

func (s *Store) reindex() {
	s.keyIdx, s.emailIdx, s.refIdx = map[string]string{}, map[string]string{}, map[string]string{}
	s.tokIdx, s.xferIdx = map[string]string{}, map[string]string{}
	for id, m := range s.d.Merchants {
		s.keyIdx[m.SecretHash] = id
		s.emailIdx[m.Email] = id
	}
	for id, t := range s.d.Txs {
		s.refIdx[t.MerchantID+"|"+t.TxRef] = id
		s.tokIdx[t.CheckoutToken] = id
	}
	for id, t := range s.d.Transfers {
		s.xferIdx[t.MerchantID+"|"+t.Reference] = id
	}
}

// markDirty must be called with s.mu held after any mutation.
func (s *Store) markDirty() {
	select {
	case s.dirty <- struct{}{}:
	default:
	}
}

func (s *Store) flusher() {
	debounce := 150 * time.Millisecond
	if s.remote != nil {
		debounce = 30 * time.Second // keeps free-tier remote stores well inside their quotas
	}
	for range s.dirty {
		time.Sleep(debounce)
		s.Flush()
	}
}

// Flush writes a snapshot to disk and/or the remote store. Safe to call
// concurrently; calls are serialised so an older snapshot never lands last.
func (s *Store) Flush() {
	if s.path == "" && s.remote == nil {
		return
	}
	s.flushMu.Lock()
	defer s.flushMu.Unlock()
	s.mu.Lock()
	b, err := json.Marshal(&s.d)
	s.mu.Unlock()
	if err != nil {
		log.Printf("snapshot marshal failed: %v", err)
		return
	}
	if s.path != "" {
		tmp := s.path + ".tmp"
		if err := os.WriteFile(tmp, b, 0o600); err == nil {
			_ = os.Rename(tmp, s.path)
		}
	}
	if s.remote != nil {
		if err := s.remote.Save(b); err != nil {
			log.Printf("remote snapshot failed (will retry on next change): %v", err)
			s.markDirtyAsync()
		}
	}
}

func (s *Store) markDirtyAsync() {
	select {
	case s.dirty <- struct{}{}:
	default:
	}
}

// ---- double-entry ledger ----

type Line struct {
	Account       string
	Debit, Credit Money
}

// postLocked records balanced entries. Caller holds s.mu.
func (s *Store) postLocked(txID, memo string, lines ...Line) error {
	var dr, cr Money
	for _, l := range lines {
		dr += l.Debit
		cr += l.Credit
	}
	if dr != cr {
		return errors.New("unbalanced ledger entry")
	}
	now := time.Now().UTC()
	for _, l := range lines {
		s.d.Seq++
		s.d.Ledger = append(s.d.Ledger, LedgerEntry{ID: s.d.Seq, TxID: txID, Account: l.Account,
			Debit: l.Debit, Credit: l.Credit, Memo: memo, CreatedAt: now})
		s.d.Balances[l.Account] += l.Credit - l.Debit
	}
	return nil
}

// Account names.
const (
	acctClearing   = "assets:provider_clearing"
	acctRevenue    = "revenue:platform_fees"
	acctPayoutHold = "liabilities:payouts_pending"
	acctPayoutDone = "assets:payouts_settled"
)

func acctMerchant(id string) string { return "merchant:" + id + ":available" }

// ---- helpers ----

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func sha(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
