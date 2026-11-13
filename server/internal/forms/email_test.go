package forms

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestConfirmationEmailRules(t *testing.T) {
	def := conditionalForm()
	resp := func(e *ConfirmationEmail) Response {
		return Response{ItemType: "task", Location: Location{SpaceID: uuid.New()}, Email: e}
	}
	good := &ConfirmationEmail{SenderName: "Acme Support", LogoURL: "https://cdn.example.test/logo.png", Greeting: "Hi {name}", Message: "We got {title}\nSecond line", Closing: "Thanks"}
	if err := ValidateResponse(def, resp(good)); err != nil {
		t.Fatalf("valid wording refused: %v", err)
	}
	bad := map[string]*ConfirmationEmail{
		"newline in name":     {SenderName: "A\r\nBcc: x@y.test"},
		"long name":           {SenderName: strings.Repeat("x", 81)},
		"control in greeting": {Greeting: "Hi\x00"},
		"long message":        {Message: strings.Repeat("x", 2001)},
		"http logo":           {LogoURL: "http://x.test/l.png"},
		"script logo":         {LogoURL: "javascript:alert(1)"},
		"logo with userinfo":  {LogoURL: "https://user:pw@x.test/l.png"},
		"hostless logo":       {LogoURL: "https:///l.png"},
	}
	for name, e := range bad {
		if ValidateResponse(def, resp(e)) == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if got := ConfirmationText("Re {title} / {form} / {name}", "T", "F", "N"); got != "Re T / F / N" {
		t.Errorf("placeholders: %q", got)
	}
}
