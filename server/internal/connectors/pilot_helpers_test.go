package connectors_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"os"
)

func getenv(k string) string { return os.Getenv(k) }

func hmacHex(secret string, body []byte) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write(body)
	return hex.EncodeToString(m.Sum(nil))
}
