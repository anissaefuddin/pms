package storage

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// The "GET Bucket (List Objects)" example of the AWS Signature V4 documentation for Amazon S3.
func TestSignV4MatchesTheAWSExample(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, "https://examplebucket.s3.amazonaws.com/?max-keys=2&prefix=J", nil)
	SignV4(r, emptySHA256, "us-east-1", "AKIAIOSFODNN7EXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", time.Date(2013, 5, 24, 0, 0, 0, 0, time.UTC))
	want := "Signature=34b48302e7b5fa45bde8084f4b7868a86f0a534bc59db6670ed5711ef69dc6f7"
	if got := r.Header.Get("Authorization"); !strings.HasSuffix(got, want) {
		t.Fatalf("Authorization = %s, want suffix %s", got, want)
	}
}
