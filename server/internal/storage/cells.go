package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

// Pydio Cells storage through its S3 gateway (bucket "io", path style), verified against Cells 5 (tahap 0):
// one request fails above about 100 MB behind the public proxy, a PUT silently replaces an existing object, conditional PUT is
// ignored, a DELETE can answer success while the file stays, and a file answers 403 for a moment after a multipart upload.

// ErrUnavailable means Pydio did not do what was asked. The message never carries the address or the folder path.
var ErrUnavailable = errors.New("pydio storage is unavailable")

const (
	cellsSecret   = "gatewaysecret" // fixed secret of the Cells gateway; the personal access token is the access key
	cellsPartSize = 50 << 20
)

// CellsTransport replaces the HTTP transport in tests, which serve a fake gateway with a self-signed certificate.
var CellsTransport http.RoundTripper

// Keys stay within this alphabet: the gateway signature needs no escaping and the path reads the same everywhere.
var cellsSegment = regexp.MustCompile(`^[A-Za-z0-9_-][A-Za-z0-9._-]{0,199}$`)

// CellsPathOK reports whether path is a usable workspace folder or object key.
func CellsPathOK(path string) bool {
	if path == "" || len(path) > 900 {
		return false
	}
	for _, seg := range strings.Split(path, "/") {
		if !cellsSegment.MatchString(seg) {
			return false
		}
	}
	return true
}

// Cells stores files under keys the caller chooses: <workspace>/<folders>/<name>.
type Cells struct {
	objects  ObjectStore
	PartSize int64
}

func NewCells(base, token string) Cells {
	client := &http.Client{Timeout: 5 * time.Minute, Transport: CellsTransport,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }} // the token never follows a redirect
	return Cells{PartSize: cellsPartSize, objects: ObjectStore{Client: client, Base: strings.TrimRight(base, "/") + "/io", Sign: func(r *http.Request, payload string) {
		SignV4(r, payload, "us-east-1", token, cellsSecret, time.Now())
	}}}
}

func (c Cells) request(method, key, payload string, body io.Reader, size int64) (*http.Response, error) {
	res, err := c.objects.do(method, key, payload, body, size)
	if err == nil || errors.Is(err, fs.ErrNotExist) {
		return res, err
	}
	var urlErr *url.Error // repeats the URL, which holds the folder path: keep only the cause
	if errors.As(err, &urlErr) {
		err = urlErr.Err
	}
	return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
}

// Put is not available: every Cells file has a place chosen from the item it belongs to.
func (c Cells) Put(io.Reader, int64) (File, error) {
	return File{}, errors.New("a Pydio file needs a path: use PutAt")
}

// PutAt writes a new file. The caller must pass a key nothing else uses, because Cells replaces without asking.
func (c Cells) PutAt(key string, src io.Reader, maxBytes int64) (File, error) {
	if !CellsPathOK(key) {
		return File{}, os.ErrInvalid
	}
	if maxBytes <= 0 {
		return File{}, ErrTooLarge
	}
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
	sum := hex.EncodeToString(hash.Sum(nil))
	if n <= c.PartSize {
		var res *http.Response
		if res, err = c.request(http.MethodPut, key, sum, io.NewSectionReader(tmp, 0, n), n); err == nil {
			res.Body.Close()
		}
	} else {
		err = c.multipart(key, tmp, n)
	}
	if err == nil {
		err = c.verify(key, n)
	}
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) { // the workspace or the gateway refused the path
			err = fmt.Errorf("%w: the folder cannot be written", ErrUnavailable)
		}
		return File{}, err
	}
	return File{Key: key, Size: n, SHA256: sum}, nil
}

// settle repeats a check while Cells catches up with a change, for about 13 seconds in all.
func settle(check func() error) error {
	err := check()
	for wait := 100 * time.Millisecond; err != nil && wait <= 6400*time.Millisecond; wait *= 2 {
		time.Sleep(wait)
		err = check()
	}
	return err
}

func (c Cells) verify(key string, size int64) error {
	return settle(func() error {
		res, err := c.request(http.MethodHead, key, emptySHA256, nil, 0)
		if err != nil {
			return err
		}
		res.Body.Close()
		if res.ContentLength != size {
			return fmt.Errorf("%w: stored size differs from the upload", ErrUnavailable)
		}
		return nil
	})
}

func (c Cells) multipart(key string, file io.ReaderAt, size int64) (err error) {
	res, err := c.request(http.MethodPost, key+"?uploads=", emptySHA256, nil, 0)
	if err != nil {
		return err
	}
	var started struct {
		UploadID string `xml:"UploadId"`
	}
	err = xml.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&started)
	res.Body.Close()
	if err != nil || started.UploadID == "" {
		return fmt.Errorf("%w: multipart upload did not start", ErrUnavailable)
	}
	upload := "uploadId=" + url.QueryEscape(started.UploadID)
	defer func() {
		if err != nil {
			if res, aerr := c.request(http.MethodDelete, key+"?"+upload, emptySHA256, nil, 0); aerr == nil {
				res.Body.Close()
			}
		}
	}()
	type part struct {
		PartNumber int
		ETag       string
	}
	var done struct {
		XMLName xml.Name `xml:"CompleteMultipartUpload"`
		Parts   []part   `xml:"Part"`
	}
	for offset, number := int64(0), 1; offset < size; offset, number = offset+c.PartSize, number+1 {
		length := min(c.PartSize, size-offset)
		hash := sha256.New()
		if _, err = io.Copy(hash, io.NewSectionReader(file, offset, length)); err != nil {
			return err
		}
		res, err = c.request(http.MethodPut, fmt.Sprintf("%s?partNumber=%d&%s", key, number, upload), hex.EncodeToString(hash.Sum(nil)), io.NewSectionReader(file, offset, length), length)
		if err != nil {
			return err
		}
		res.Body.Close()
		done.Parts = append(done.Parts, part{number, res.Header.Get("ETag")})
	}
	body, err := xml.Marshal(done)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(body)
	res, err = c.request(http.MethodPost, key+"?"+upload, hex.EncodeToString(digest[:]), strings.NewReader(string(body)), int64(len(body)))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	answer, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err == nil && strings.Contains(string(answer), "<Error>") { // S3 may report a failed completion with status 200
		err = fmt.Errorf("%w: multipart upload did not complete", ErrUnavailable)
	}
	return err
}

func (c Cells) Open(key string) (Reader, error) {
	if !CellsPathOK(key) {
		return nil, os.ErrInvalid
	}
	res, err := c.request(http.MethodHead, key, emptySHA256, nil, 0)
	if err != nil {
		return nil, err
	}
	res.Body.Close()
	return &objectReader{s: c.objects, key: key, size: res.ContentLength}, nil
}

// Remove deletes and then looks: success is only what can be seen.
func (c Cells) Remove(key string) error {
	if !CellsPathOK(key) {
		return os.ErrInvalid
	}
	res, err := c.request(http.MethodDelete, key, emptySHA256, nil, 0)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	res.Body.Close()
	return settle(func() error {
		res, err := c.request(http.MethodHead, key, emptySHA256, nil, 0)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		res.Body.Close()
		return fmt.Errorf("%w: the file is still there after delete", ErrUnavailable)
	})
}
