package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"syscall"
	"time"
)

func sortTx(l []*Transaction) {
	sort.Slice(l, func(i, j int) bool { return l[i].CreatedAt.After(l[j].CreatedAt) })
}
func sortTransfers(l []*Transfer) {
	sort.Slice(l, func(i, j int) bool { return l[i].CreatedAt.After(l[j].CreatedAt) })
}

func validateWebhookURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || len(raw) > 500 {
		return errors.New("URL must be http(s) with a host")
	}
	return nil
}

func sign(secret string, ts int64, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(ts, 10) + "."))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// SSRF guard: refuse to connect to internal addresses unless explicitly allowed.
func (a *App) webhookClient() *http.Client {
	d := &net.Dialer{Timeout: 5 * time.Second, Control: func(network, address string, _ syscall.RawConn) error {
		if a.cfg.AllowPrivateWebhooks {
			return nil
		}
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return err
		}
		ip := net.ParseIP(host)
		if ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
			ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
			return errors.New("webhook target resolves to a private address")
		}
		return nil
	}}
	return &http.Client{
		Timeout:       10 * time.Second,
		Transport:     &http.Transport{DialContext: d.DialContext, DisableKeepAlives: true},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// RunWebhooks delivers queued webhooks with retries and exponential backoff.
// State lives in the store, so pending deliveries survive restarts.
func (a *App) RunWebhooks(stop <-chan struct{}) {
	client := a.webhookClient()
	tick := time.NewTicker(300 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-stop:
			return
		case <-tick.C:
		}
		var due []Delivery
		a.s.mu.Lock()
		now := time.Now()
		for _, d := range a.s.d.Deliveries {
			if d.Status == "pending" && !d.NextAttempt.After(now) {
				d.NextAttempt = now.Add(30 * time.Second) // lease while in flight
				due = append(due, *d)
			}
		}
		a.s.mu.Unlock()
		for _, d := range due {
			go a.deliver(client, d)
		}
	}
}

func (a *App) deliver(c *http.Client, d Delivery) {
	body := []byte(d.Payload)
	ts := time.Now().Unix()
	req, _ := http.NewRequest("POST", d.URL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "TechETPay-Webhooks/1.0")
	req.Header.Set("X-TechET-Timestamp", strconv.FormatInt(ts, 10))
	req.Header.Set("X-TechET-Signature", sign(d.Secret, ts, body))
	req.Header.Set("X-TechET-Delivery", d.ID)

	code, errMsg := 0, ""
	resp, err := c.Do(req)
	if err != nil {
		errMsg = err.Error()
	} else {
		code = resp.StatusCode
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		resp.Body.Close()
		if code < 200 || code >= 300 {
			errMsg = fmt.Sprintf("endpoint returned HTTP %d", code)
		}
	}

	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	cur := a.s.d.Deliveries[d.ID]
	if cur == nil {
		return
	}
	cur.Attempts++
	cur.LastCode, cur.LastError = code, errMsg
	switch {
	case errMsg == "":
		cur.Status = "delivered"
	case cur.Attempts > len(a.cfg.Backoff):
		cur.Status = "failed"
	default:
		cur.NextAttempt = time.Now().Add(a.cfg.Backoff[cur.Attempts-1])
	}
	a.s.markDirty()
}

func (a *App) ListDeliveries(merchantID string) []map[string]any {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	var l []*Delivery
	for _, d := range a.s.d.Deliveries {
		if d.MerchantID == merchantID {
			l = append(l, d)
		}
	}
	sort.Slice(l, func(i, j int) bool { return l[i].CreatedAt.After(l[j].CreatedAt) })
	if len(l) > 50 {
		l = l[:50]
	}
	out := []map[string]any{}
	for _, d := range l {
		out = append(out, map[string]any{"id": d.ID, "event": d.Event, "status": d.Status, "attempts": d.Attempts,
			"last_code": d.LastCode, "last_error": d.LastError, "created_at": d.CreatedAt})
	}
	return out
}
