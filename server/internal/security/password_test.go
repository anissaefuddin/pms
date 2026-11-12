package security

import "testing"

func TestPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(hash, "correct horse battery staple") {
		t.Fatal("valid password rejected")
	}
	if CheckPassword(hash, "wrong password") {
		t.Fatal("invalid password accepted")
	}
}

func TestShortPasswordRejected(t *testing.T) {
	if _, err := HashPassword("short"); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestCommonPasswordsRejectedAfterNormalizing(t *testing.T) {
	for _, bad := range []string{"password", "Password", "  PASSWORD  ", "12345678", "QWERTY123", "iloveyou", "password1", " 123123123"} {
		if err := ValidatePassword(bad); err != ErrPasswordCommon {
			t.Fatalf("%q: got %v, want ErrPasswordCommon", bad, err)
		}
	}
	for _, ok := range []string{"password12", "correct horse battery", "Tr0ub4dor&3"} {
		if err := ValidatePassword(ok); err != nil {
			t.Fatalf("%q rejected: %v", ok, err)
		}
	}
	if err := ValidatePassword("pass"); err != ErrPasswordTooShort {
		t.Fatalf("short password: %v", err)
	}
}

func TestPasswordLengthCountsCharactersNotBytes(t *testing.T) {
	if err := ValidatePassword("密码密码"); err != ErrPasswordTooShort { // 12 bytes, 4 characters
		t.Fatalf("4 characters accepted: %v", err)
	}
	if err := ValidatePassword("密码密码密码密码"); err != nil {
		t.Fatalf("8 characters rejected: %v", err)
	}
}
