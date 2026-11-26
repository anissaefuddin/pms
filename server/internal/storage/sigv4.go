package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// SignV4 signs r with AWS Signature V4 for service s3, signing host, x-amz-content-sha256 and x-amz-date. The query is
// put in canonical form (sorted, %20 for spaces) and written back to r.URL, so what is sent is what was signed.
// payload is the hex SHA-256 of the body.
func SignV4(r *http.Request, payload, region, accessKey, secretKey string, now time.Time) {
	now = now.UTC()
	date, stamp := now.Format("20060102"), now.Format("20060102T150405Z")
	r.URL.RawQuery = strings.ReplaceAll(r.URL.Query().Encode(), "+", "%20")
	r.Header.Set("x-amz-date", stamp)
	r.Header.Set("x-amz-content-sha256", payload)
	path := r.URL.EscapedPath()
	if path == "" {
		path = "/"
	}
	canonical := fmt.Sprintf("%s\n%s\n%s\nhost:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n\nhost;x-amz-content-sha256;x-amz-date\n%s",
		r.Method, path, r.URL.RawQuery, r.URL.Host, payload, stamp, payload)
	scope := date + "/" + region + "/s3/aws4_request"
	digest := sha256.Sum256([]byte(canonical))
	toSign := "AWS4-HMAC-SHA256\n" + stamp + "\n" + scope + "\n" + hex.EncodeToString(digest[:])
	k := []byte("AWS4" + secretKey)
	for _, part := range []string{date, region, "s3", "aws4_request"} {
		k = hmacSHA256(k, part)
	}
	r.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+accessKey+"/"+scope+
		", SignedHeaders=host;x-amz-content-sha256;x-amz-date, Signature="+hex.EncodeToString(hmacSHA256(k, toSign)))
}
