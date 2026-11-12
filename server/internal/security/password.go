package security

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

const (
	memory    = 64 * 1024
	timeCost  = 3
	threads   = 2
	keyLength = 32
)

const MinPasswordLength = 8 // api/openapi.yaml minLength

var ErrPasswordTooShort = fmt.Errorf("password must be at least %d characters", MinPasswordLength)

var ErrPasswordCommon = errors.New("password is too common")

// commonPasswords are the ten worst passwords of at least 8 characters (decision D-14, WORKFLOWS §4).
// shortcut: ten entries only; swap in a bigger list (for example the top 10,000) when a client asks for a stricter policy.
var commonPasswords = map[string]struct{}{
	"password": {}, "password1": {}, "12345678": {}, "123456789": {}, "1234567890": {},
	"qwerty123": {}, "qwertyuiop": {}, "iloveyou": {}, "11111111": {}, "123123123": {},
}

// ValidatePassword is the cheap check; call it before anything that costs an argon2 run.
// Length counts characters of the raw input; the common-password check ignores case and surrounding spaces.
func ValidatePassword(password string) error {
	if utf8.RuneCountInString(password) < MinPasswordLength { // characters, like openapi minLength
		return ErrPasswordTooShort
	}
	if _, bad := commonPasswords[strings.ToLower(strings.TrimSpace(password))]; bad {
		return ErrPasswordCommon
	}
	return nil
}

// slots caps concurrent argon2 runs: each uses 64 MiB, so unbounded parallelism lets requests exhaust memory.
// shortcut: blocking acquire with no timeout, fine for one replica; add a context-aware wait if login latency under load matters.
var slots = make(chan struct{}, 2)

func HashPassword(password string) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}
	slots <- struct{}{}
	defer func() { <-slots }()
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, timeCost, memory, threads, keyLength)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", memory, timeCost, threads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash)), nil
}

func CheckPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var version int
	var m uint32
	var t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false
	}
	if m > memory*2 || t > timeCost*2 || p > 8 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	slots <- struct{}{}
	defer func() { <-slots }()
	got := argon2.IDKey([]byte(password), salt, t, m, p, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

func Token() (plain string, hash []byte, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", nil, err
	}
	plain = base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(plain))
	return plain, sum[:], nil
}

func TokenHash(plain string) []byte {
	sum := sha256.Sum256([]byte(plain))
	return sum[:]
}
