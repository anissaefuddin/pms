package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

var ErrTooLarge = errors.New("attachment exceeds upload limit")

type File struct {
	Key    string
	Size   int64
	SHA256 string
}

type Storage interface {
	Put(io.Reader, int64) (File, error)
	Open(string) (Reader, error)
	Remove(string) error
}

type Reader interface {
	io.ReadCloser
	io.ReaderAt
}

// shortcut: keep files on the mounted local volume until a remote adapter is needed.
type LocalStorage struct{ Dir string }

func (s LocalStorage) Put(src io.Reader, maxBytes int64) (File, error) {
	if maxBytes <= 0 {
		return File{}, ErrTooLarge
	}
	root, err := os.OpenRoot(s.Dir)
	if err != nil {
		return File{}, err
	}
	defer root.Close()
	id, err := uuid.NewRandom()
	if err != nil {
		return File{}, err
	}
	value := id.String()
	dir := filepath.Join(value[:2], value[2:4])
	if err := root.MkdirAll(dir, 0700); err != nil {
		return File{}, err
	}
	key := filepath.Join(dir, value)
	out, err := root.OpenFile(key, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return File{}, err
	}
	hash := sha256.New()
	n, err := copyLimited(io.MultiWriter(out, hash), src, maxBytes)
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return File{}, errors.Join(err, root.Remove(key))
	}
	return File{Key: key, Size: n, SHA256: hex.EncodeToString(hash.Sum(nil))}, nil
}

func (s LocalStorage) Open(key string) (Reader, error) {
	if !validKey(key) {
		return nil, os.ErrInvalid
	}
	root, err := os.OpenRoot(s.Dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return root.Open(key)
}

func (s LocalStorage) Remove(key string) error {
	if !validKey(key) {
		return os.ErrInvalid
	}
	root, err := os.OpenRoot(s.Dir)
	if err != nil {
		return err
	}
	defer root.Close()
	return root.Remove(key)
}

func validKey(key string) bool {
	value := filepath.Base(key)
	id, err := uuid.Parse(value)
	return err == nil && id.String() == value && key == filepath.Join(value[:2], value[2:4], value)
}

// Keys lists stored files last written before cutoff. A missing directory is empty, not an error.
func (s LocalStorage) Keys(cutoff time.Time) ([]string, error) {
	var keys []string
	err := filepath.WalkDir(s.Dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, relErr := filepath.Rel(s.Dir, path)
		info, infoErr := d.Info()
		if relErr == nil && infoErr == nil && validKey(rel) && info.ModTime().Before(cutoff) {
			keys = append(keys, rel)
		}
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return keys, err
}

// copyLimited copies at most max bytes and fails with ErrTooLarge when src has more.
func copyLimited(dst io.Writer, src io.Reader, max int64) (int64, error) {
	reader := &io.LimitedReader{R: src, N: max}
	n, err := io.Copy(dst, reader)
	if err == nil && reader.N == 0 {
		var extra [1]byte
		switch _, readErr := io.ReadFull(src, extra[:]); {
		case readErr == nil:
			err = ErrTooLarge
		case readErr != io.EOF:
			err = readErr
		}
	}
	return n, err
}
