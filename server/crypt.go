package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
)

// Vault encrypts provider credentials (AES-256-GCM) before they are stored.
// The key comes from TECHET_SECRET_KEY, which lives only in the server's
// environment: a stolen database snapshot or backup is useless without it.
type Vault struct{ aead cipher.AEAD }

func newVault(secret string) (*Vault, error) {
	if len(secret) < 16 {
		return nil, errors.New("TECHET_SECRET_KEY must be at least 16 characters")
	}
	key, err := pbkdf2.Key(sha256.New, secret, []byte("xidex-vault-v1"), 100_000, 32)
	if err != nil {
		return nil, err
	}
	blk, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	g, err := cipher.NewGCM(blk)
	if err != nil {
		return nil, err
	}
	return &Vault{aead: g}, nil
}

func (v *Vault) Seal(plain string) (string, error) {
	nonce := make([]byte, v.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ct := v.aead.Seal(nil, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(append(nonce, ct...)), nil
}

func (v *Vault) Open(sealed string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil || len(raw) < v.aead.NonceSize() {
		return "", errors.New("stored credential is corrupted")
	}
	n := v.aead.NonceSize()
	pt, err := v.aead.Open(nil, raw[:n], raw[n:], nil)
	if err != nil {
		return "", errors.New("cannot decrypt stored credentials: TECHET_SECRET_KEY changed?")
	}
	return string(pt), nil
}
