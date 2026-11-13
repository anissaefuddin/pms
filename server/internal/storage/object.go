package storage

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/google/uuid"
)

// ObjectStore keeps files as objects under Base (PUT/GET/HEAD/DELETE <Base>/<key>), the same key layout as LocalStorage.
// Sign authenticates one request; payload is the hex SHA-256 of its body. S3 and Pydio Cells differ only in Sign and Base.
type ObjectStore struct {
	Client *http.Client
	Base   string
	Sign   func(r *http.Request, payload string)
}

const emptySHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

// S3 signs with AWS Signature V4, path style (works with MinIO). Base is endpoint/bucket[/prefix].
func S3(endpointBucket, region, accessKey, secretKey string) ObjectStore {
	return ObjectStore{Client: http.DefaultClient, Base: endpointBucket, Sign: func(r *http.Request, payload string) {
		now := time.Now().UTC()
		date, stamp := now.Format("20060102"), now.Format("20060102T150405Z")
		r.Header.Set("x-amz-date", stamp)
		r.Header.Set("x-amz-content-sha256", payload)
		canonical := fmt.Sprintf("%s\n%s\n\nhost:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n\nhost;x-amz-content-sha256;x-amz-date\n%s",
			r.Method, r.URL.EscapedPath(), r.URL.Host, payload, stamp, payload)
		scope := date + "/" + region + "/s3/aws4_request"
		digest := sha256.Sum256([]byte(canonical))
		toSign := "AWS4-HMAC-SHA256\n" + stamp + "\n" + scope + "\n" + hex.EncodeToString(digest[:])
		k := []byte("AWS4" + secretKey)
		for _, part := range []string{date, region, "s3", "aws4_request"} {
			k = hmacSHA256(k, part)
		}
		r.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+accessKey+"/"+scope+
			", SignedHeaders=host;x-amz-content-sha256;x-amz-date, Signature="+hex.EncodeToString(hmacSHA256(k, toSign)))
	}}
}

// Pydio talks to a Cells data-source folder through its S3-compatible gateway (Base = https://cells/io/<datasource>/<folder>)
// with a personal access token.
func Pydio(base, token string) ObjectStore {
	return ObjectStore{Client: http.DefaultClient, Base: base, Sign: func(r *http.Request, _ string) {
		r.Header.Set("Authorization", "Bearer "+token)
	}}
}

func hmacSHA256(key []byte, msg string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(msg))
	return h.Sum(nil)
}

func (s ObjectStore) do(method, key, payload string, body io.Reader, size int64, header ...string) (*http.Response, error) {
	u, err := url.Parse(s.Base + "/" + key)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(method, u.String(), body)
	if err != nil {
		return nil, err
	}
	req.ContentLength = size
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	s.Sign(req, payload)
	res, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}
	switch {
	case res.StatusCode == http.StatusNotFound:
		res.Body.Close()
		return nil, fs.ErrNotExist
	case res.StatusCode >= 300:
		res.Body.Close()
		return nil, fmt.Errorf("object store %s: %s", method, res.Status)
	}
	return res, nil
}

func (s ObjectStore) Put(src io.Reader, maxBytes int64) (File, error) {
	if maxBytes <= 0 {
		return File{}, ErrTooLarge
	}
	// The object needs its size and digest up front, so the upload is staged in a temp file first.
	tmp, err := os.CreateTemp("", "pms-upload-*")
	if err != nil {
		return File{}, err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	hash := sha256.New()
	n, err := copyLimited(io.MultiWriter(tmp, hash), src, maxBytes)
	if err != nil {
		return File{}, err
	}
	if _, err = tmp.Seek(0, io.SeekStart); err != nil {
		return File{}, err
	}
	id, err := uuid.NewRandom()
	if err != nil {
		return File{}, err
	}
	value := id.String()
	key := value[:2] + "/" + value[2:4] + "/" + value
	sum := hex.EncodeToString(hash.Sum(nil))
	res, err := s.do(http.MethodPut, key, sum, tmp, n)
	if err != nil {
		return File{}, err
	}
	res.Body.Close()
	return File{Key: key, Size: n, SHA256: sum}, nil
}

func (s ObjectStore) Open(key string) (Reader, error) {
	if !validKey(key) {
		return nil, os.ErrInvalid
	}
	res, err := s.do(http.MethodHead, key, emptySHA256, nil, 0)
	if err != nil {
		return nil, err
	}
	res.Body.Close()
	return &objectReader{s: s, key: key, size: res.ContentLength}, nil
}

func (s ObjectStore) Remove(key string) error {
	if !validKey(key) {
		return os.ErrInvalid
	}
	res, err := s.do(http.MethodDelete, key, emptySHA256, nil, 0)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err == nil {
		res.Body.Close()
	}
	return err
}

type objectReader struct {
	s    ObjectStore
	key  string
	size int64
	body io.ReadCloser
}

func (o *objectReader) Read(p []byte) (int, error) {
	if o.body == nil {
		res, err := o.s.do(http.MethodGet, o.key, emptySHA256, nil, 0)
		if err != nil {
			return 0, err
		}
		o.body = res.Body
	}
	return o.body.Read(p)
}

func (o *objectReader) ReadAt(p []byte, off int64) (int, error) {
	if off >= o.size {
		return 0, io.EOF
	}
	res, err := o.s.do(http.MethodGet, o.key, emptySHA256, nil, 0, "Range", fmt.Sprintf("bytes=%d-%d", off, off+int64(len(p))-1))
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	n, err := io.ReadFull(res.Body, p)
	if err == io.ErrUnexpectedEOF {
		err = io.EOF
	}
	return n, err
}

func (o *objectReader) Close() error {
	if o.body != nil {
		return o.body.Close()
	}
	return nil
}
