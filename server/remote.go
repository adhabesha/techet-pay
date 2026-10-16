package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Remote is an optional off-box snapshot store, used on hosts whose disk is
// wiped on restart (e.g. free Render instances). It speaks the Upstash Redis
// REST protocol: POST a JSON command array to the database URL.
// Free Upstash databases need no credit card.
type Remote struct {
	URL, Token string
	client     *http.Client
}

const remoteKey = "techet:state:v1"

func newRemote(url, token string) *Remote {
	url, token = strings.TrimRight(strings.TrimSpace(url), "/"), strings.TrimSpace(token)
	if url == "" || token == "" {
		return nil
	}
	return &Remote{URL: url, Token: token, client: &http.Client{Timeout: 15 * time.Second}}
}

func (r *Remote) do(cmd ...string) (*string, error) {
	body, _ := json.Marshal(cmd)
	req, _ := http.NewRequest("POST", r.URL, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+r.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	var out struct {
		Result *string `json:"result"`
		Error  string  `json:"error"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("remote store: unexpected response (HTTP %d)", resp.StatusCode)
	}
	if out.Error != "" {
		return nil, errors.New("remote store: " + out.Error)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("remote store: HTTP %d", resp.StatusCode)
	}
	return out.Result, nil
}

// Load returns nil (no error) when nothing has been saved yet.
func (r *Remote) Load() ([]byte, error) {
	res, err := r.do("GET", remoteKey)
	if err != nil || res == nil {
		return nil, err
	}
	return []byte(*res), nil
}

func (r *Remote) Save(b []byte) error {
	_, err := r.do("SET", remoteKey, string(b))
	return err
}
