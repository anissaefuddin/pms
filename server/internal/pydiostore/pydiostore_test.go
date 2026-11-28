package pydiostore

import (
	"strings"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/storage"
	"github.com/google/uuid"
)

func TestFileNamesBecomeSafeSegments(t *testing.T) {
	for in, want := range map[string]string{
		"Laporan Akhir (rev 2).pdf":       "Laporan-Akhir-rev-2.pdf",
		"résumé – nilai+100%.docx":        "r-sum-nilai-100.docx",
		".pydio":                          "pydio",
		"..":                              "file",
		"???.PNG":                         "file.PNG",
		"noext":                           "noext",
		strings.Repeat("a", 300) + ".txt": strings.Repeat("a", 120) + ".txt",
	} {
		got := FileName(in)
		if got != want {
			t.Errorf("FileName(%q) = %q, want %q", in, got, want)
		}
		if !storage.CellsPathOK(Key("pms-files/ACME/P1", uuid.New(), uuid.New(), in)) {
			t.Errorf("%q gives a key the adapter refuses", in)
		}
	}
}
