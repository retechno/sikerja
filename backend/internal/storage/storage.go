// Package storage menyimpan lampiran di disk privat (di luar web root).
package storage

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Store adalah abstraksi penyimpanan berkas; path selalu relatif terhadap root.
type Store interface {
	// Save menulis isi r ke <dir>/<acak><ext> dan mengembalikan path relatif.
	Save(dir, ext string, r io.Reader) (string, error)
	Open(rel string) (*os.File, error)
	Delete(rel string) error
}

type Local struct{ root string }

func NewLocal(root string) (*Local, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o750); err != nil {
		return nil, err
	}
	return &Local{root: abs}, nil
}

var ErrBadPath = errors.New("path berkas tidak valid")

// resolve memastikan path berada di dalam root (anti path traversal).
func (l *Local) resolve(rel string) (string, error) {
	if rel == "" || strings.Contains(rel, "..") || filepath.IsAbs(rel) {
		return "", ErrBadPath
	}
	full := filepath.Join(l.root, filepath.FromSlash(rel))
	if !strings.HasPrefix(full, l.root+string(filepath.Separator)) {
		return "", ErrBadPath
	}
	return full, nil
}

func (l *Local) Save(dir, ext string, r io.Reader) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	rel := fmt.Sprintf("%s/%s%s", strings.Trim(dir, "/"), hex.EncodeToString(b), strings.ToLower(ext))
	full, err := l.resolve(rel)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		return "", err
	}
	f, err := os.OpenFile(full, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(f, r); err != nil {
		_ = f.Close()
		_ = os.Remove(full)
		return "", err
	}
	return rel, f.Close()
}

func (l *Local) Open(rel string) (*os.File, error) {
	full, err := l.resolve(rel)
	if err != nil {
		return nil, err
	}
	return os.Open(full)
}

func (l *Local) Delete(rel string) error {
	full, err := l.resolve(rel)
	if err != nil {
		return err
	}
	if err := os.Remove(full); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
