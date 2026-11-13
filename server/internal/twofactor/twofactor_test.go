package twofactor

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

// RFC 6238 appendix B uses the ASCII secret "12345678901234567890"; its 6-digit codes are the last 6 digits of the table.
func TestCodeMatchesRFC6238(t *testing.T) {
	secret := b32.EncodeToString([]byte("12345678901234567890"))
	for at, want := range map[int64]string{59: "287082", 1111111109: "081804", 1111111111: "050471", 1234567890: "005924", 2000000000: "279037"} {
		got, err := Code(secret, StepAt(time.Unix(at, 0)))
		if err != nil || got != want {
			t.Errorf("t=%d: code %s (%v), want %s", at, got, err, want)
		}
	}
}

func TestVerifyWindowAndReplay(t *testing.T) {
	secret, _ := NewSecret()
	now := time.Unix(1_700_000_000, 0)
	step := StepAt(now)
	at := func(s int64) string { c, _ := Code(secret, s); return c }
	if got, ok := Verify(secret, at(step), now, 0); !ok || got != step {
		t.Fatalf("current step: %d %v", got, ok)
	}
	for _, s := range []int64{step - 1, step + 1} {
		if _, ok := Verify(secret, at(s), now, 0); !ok {
			t.Errorf("one step of drift (%d) must be accepted", s-step)
		}
	}
	for _, s := range []int64{step - 2, step + 2} {
		if _, ok := Verify(secret, at(s), now, 0); ok {
			t.Errorf("two steps of drift (%d) must be refused", s-step)
		}
	}
	if _, ok := Verify(secret, at(step), now, step); ok {
		t.Error("a step that was already used must be refused")
	}
	if _, ok := Verify(secret, at(step), now, step-1); !ok {
		t.Error("a newer step than the last used one is fine")
	}
	for _, bad := range []string{"", "12345", "1234567", "abcdef", "000000"} {
		if _, ok := Verify(secret, bad, now, 0); ok && bad != at(step) && bad != at(step-1) && bad != at(step+1) {
			t.Errorf("%q accepted", bad)
		}
	}
	if _, ok := Verify(secret, at(step)[:3]+" "+at(step)[3:], now, 0); !ok {
		t.Error("a code typed with a space is accepted")
	}
}

func TestEncryptionRotation(t *testing.T) {
	oldKey, _ := ParseKey(base64.StdEncoding.EncodeToString([]byte(strings.Repeat("a", 32))))
	newKey, _ := ParseKey(base64.StdEncoding.EncodeToString([]byte(strings.Repeat("b", 32))))
	stored, err := Keys{Current: oldKey}.Encrypt("JBSWY3DPEHPK3PXP", "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if plain, old, err := (Keys{Current: oldKey}).Decrypt(stored, "user-1"); err != nil || plain != "JBSWY3DPEHPK3PXP" || old {
		t.Fatalf("current key: %q old=%v %v", plain, old, err)
	}
	rotated := Keys{Current: newKey, Previous: oldKey}
	if plain, old, err := rotated.Decrypt(stored, "user-1"); err != nil || plain != "JBSWY3DPEHPK3PXP" || !old {
		t.Fatalf("previous key must still decrypt and flag re-encryption: %q old=%v %v", plain, old, err)
	}
	if _, _, err := (Keys{Current: newKey}).Decrypt(stored, "user-1"); err == nil {
		t.Error("an unknown key must not decrypt")
	}
	if _, _, err := (Keys{Current: oldKey}).Decrypt(stored, "user-2"); err == nil {
		t.Error("a secret copied to another user must not decrypt")
	}
	if _, err := (Keys{}).Encrypt("x", "u"); err != ErrNotConfigured {
		t.Errorf("no key: %v", err)
	}
	a, _ := Keys{Current: oldKey}.Encrypt("same", "u")
	b, _ := Keys{Current: oldKey}.Encrypt("same", "u")
	if a == b {
		t.Error("each encryption must use a fresh nonce")
	}
}

func TestParseKeyAndBackupCodes(t *testing.T) {
	for _, bad := range []string{"", "short", base64.StdEncoding.EncodeToString([]byte("too short"))} {
		if _, err := ParseKey(bad); err == nil {
			t.Errorf("%q accepted as a key", bad)
		}
	}
	codes, err := BackupCodes()
	if err != nil || len(codes) != BackupCount {
		t.Fatal(err, len(codes))
	}
	seen := map[string]bool{}
	for _, c := range codes {
		if len(c) != 11 || c[5] != '-' || seen[c] {
			t.Fatalf("bad or repeated backup code %q", c)
		}
		seen[c] = true
		if string(HashBackup(strings.ToLower(c))) != string(HashBackup(c)) {
			t.Fatal("backup codes are compared case-insensitively")
		}
	}
	if u := URI("PMS", "a@b.c", "SECRET"); !strings.HasPrefix(u, "otpauth://totp/PMS:a@b.c?") || !strings.Contains(u, "secret=SECRET") {
		t.Fatal(u)
	}
}
