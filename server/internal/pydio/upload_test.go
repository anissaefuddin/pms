package pydio

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestUploadLimitsCheck(t *testing.T) {
	const mb = int64(1) << 20
	all := UploadLimits{MaxMB: DefaultMaxUploadMB}
	some := UploadLimits{MaxMB: 1, Extensions: []string{"pdf", "gz"}}
	for _, c := range []struct {
		limits UploadLimits
		name   string
		size   int64
		want   error
	}{
		{all, "video.mp4", 100 * mb, nil},
		{all, "video.mp4", 100*mb + 1, ErrUploadTooLarge},
		{all, "no-extension", 0, nil},
		{all, "x.bin", -1, ErrUploadTooLarge},
		{some, "Report.PDF", mb, nil},
		{some, "backup.tar.gz", 1, nil},
		{some, "big.pdf", mb + 1, ErrUploadTooLarge},
		{some, "run.exe", 1, ErrUploadType},
		{some, "report.pdf.exe", 1, ErrUploadType},
		{some, "pdf", 1, ErrUploadType},
		{some, "report.pdf.", 1, ErrUploadType},
		{some, "folder.pdf/run", 1, ErrUploadType},
		{UploadLimits{}, "x.txt", 1, ErrUploadTooLarge},
	} {
		if got := c.limits.Check(c.name, c.size); !errors.Is(got, c.want) {
			t.Errorf("Check(%q, %d) with %+v = %v, want %v", c.name, c.size, c.limits, got, c.want)
		}
	}
}

func TestCleanExtensions(t *testing.T) {
	got, err := CleanExtensions([]string{" .PDF ", "pdf", "docx", "7z"})
	if err != nil || !slices.Equal(got, []string{"pdf", "docx", "7z"}) {
		t.Fatalf("got %v, %v", got, err)
	}
	if got, err := CleanExtensions(nil); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("empty list: %#v, %v", got, err)
	}
	for _, bad := range []string{"", ".", "..pdf", "tar.gz", "*", "*.pdf", "application/pdf", "a b", "pdf,docx", strings.Repeat("a", 21)} {
		if _, err := CleanExtensions([]string{bad}); err == nil {
			t.Errorf("%q must be refused", bad)
		}
	}
	many := make([]string, MaxExtensions+1)
	for i := range many {
		many[i] = fmt.Sprintf("e%d", i)
	}
	if _, err := CleanExtensions(many); err == nil {
		t.Error("too many extensions must be refused")
	}
	if _, err := CleanExtensions(many[:MaxExtensions]); err != nil {
		t.Error(err)
	}
}
