// Package twofactor implements TOTP (RFC 6238: HMAC-SHA1, 30-second steps, 6 digits), encryption of the stored secret,
// and one-time backup codes. It has no database or HTTP code.
package twofactor

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	Step        = 30 * time.Second
	Digits      = 6
	BackupCount = 10
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// NewSecret returns a random 160-bit secret in the base32 form authenticator apps expect.
func NewSecret() (string, error) {
	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return b32.EncodeToString(raw), nil
}

// URI is the otpauth:// link shown as a QR code.
func URI(issuer, account, secret string) string {
	label := url.PathEscape(issuer) + ":" + url.PathEscape(account)
	q := url.Values{"secret": {secret}, "issuer": {issuer}, "algorithm": {"SHA1"}, "digits": {"6"}, "period": {"30"}}
	return "otpauth://totp/" + label + "?" + q.Encode()
}

// Code returns the code for one time step.
func Code(secret string, step int64) (string, error) {
	key, err := b32.DecodeString(strings.ToUpper(secret))
	if err != nil {
		return "", err
	}
	var counter [8]byte
	binary.BigEndian.PutUint64(counter[:], uint64(step))
	mac := hmac.New(sha1.New, key)
	mac.Write(counter[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	value := (binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff) % 1_000_000
	return fmt.Sprintf("%06d", value), nil
}

// StepAt is the time step number of an instant.
func StepAt(t time.Time) int64 { return t.Unix() / int64(Step/time.Second) }

// Verify checks a code against the steps around now (one step of clock drift either way) and returns the step it
// matched. Steps at or before lastStep are refused, so a code cannot be used twice.
func Verify(secret, code string, now time.Time, lastStep int64) (int64, bool) {
	code = strings.NewReplacer(" ", "", "-", "").Replace(code)
	if len(code) != Digits {
		return 0, false
	}
	current := StepAt(now)
	matched := int64(0)
	for _, step := range []int64{current - 1, current, current + 1} {
		want, err := Code(secret, step)
		if err != nil {
			return 0, false
		}
		// Compare every candidate so the time taken does not show which one matched.
		if subtle.ConstantTimeCompare([]byte(want), []byte(code)) == 1 && step > lastStep && step > matched {
			matched = step
		}
	}
	return matched, matched != 0
}

// Keys encrypt secrets with the current key and decrypt with the current or the previous key, which makes rotation
// a configuration change: set the old key as previous, then the next successful login re-encrypts with the new one.
type Keys struct{ Current, Previous []byte }

// ParseKey reads a base64 key of exactly 32 bytes.
func ParseKey(value string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil || len(key) != 32 {
		return nil, errors.New("must be 32 random bytes encoded as base64 (for example `openssl rand -base64 32`)")
	}
	return key, nil
}

func gcm(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// Encrypt returns "v1:" + base64(nonce || ciphertext). The user id is bound in as additional data, so a stored
// secret copied to another user's row does not decrypt.
func (k Keys) Encrypt(plain, userID string) (string, error) {
	if len(k.Current) != 32 {
		return "", ErrNotConfigured
	}
	aead, err := gcm(k.Current)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return "v1:" + base64.StdEncoding.EncodeToString(aead.Seal(nonce, nonce, []byte(plain), []byte(userID))), nil
}

// Decrypt returns the secret and whether it was decrypted with the previous key (the caller should re-encrypt it).
func (k Keys) Decrypt(stored, userID string) (plain string, old bool, err error) {
	raw, ok := strings.CutPrefix(stored, "v1:")
	if !ok {
		return "", false, errors.New("unknown secret format")
	}
	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return "", false, err
	}
	for i, key := range [][]byte{k.Current, k.Previous} {
		if len(key) != 32 {
			continue
		}
		aead, err := gcm(key)
		if err != nil || len(data) < aead.NonceSize() {
			continue
		}
		if out, err := aead.Open(nil, data[:aead.NonceSize()], data[aead.NonceSize():], []byte(userID)); err == nil {
			return string(out), i == 1, nil
		}
	}
	return "", false, errors.New("secret cannot be decrypted with the configured keys")
}

var ErrNotConfigured = errors.New("two-factor authentication is not configured")

// BackupCodes returns BackupCount fresh codes like "K7QD2-M4XPA" (50 random bits each).
func BackupCodes() ([]string, error) {
	out := make([]string, BackupCount)
	for i := range out {
		raw := make([]byte, 7)
		if _, err := rand.Read(raw); err != nil {
			return nil, err
		}
		text := b32.EncodeToString(raw)[:10]
		out[i] = text[:5] + "-" + text[5:]
	}
	return out, nil
}

// NormalizeBackup makes a typed backup code comparable: no spaces or dashes, upper case.
func NormalizeBackup(code string) string {
	return strings.ToUpper(strings.NewReplacer(" ", "", "-", "").Replace(code))
}

// HashBackup is the stored form of a backup code. The codes are random, so a plain hash is enough.
func HashBackup(code string) []byte {
	sum := sha256.Sum256([]byte(NormalizeBackup(code)))
	return sum[:]
}
