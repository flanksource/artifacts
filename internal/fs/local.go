package fs

import (
	gocontext "context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// localFS implements FilesystemRW for local filesystem
type localFS struct {
	base string
}

func NewLocalFS(base string) *localFS {
	return &localFS{base: base}
}

func (t *localFS) Close() error {
	return nil
}

func (t *localFS) ReadDir(name string) ([]os.FileInfo, error) {
	return nil, nil // TODO:
}

func (t *localFS) Stat(name string) (os.FileInfo, error) {
	return os.Stat(filepath.Join(t.base, name))
}

func (t *localFS) Read(ctx gocontext.Context, path string) (io.ReadCloser, error) {
	return os.Open(filepath.Join(t.base, path))
}

func (t *localFS) Write(ctx gocontext.Context, path string, data io.Reader) (os.FileInfo, error) {
	fullpath := filepath.Join(t.base, path)

	// Ensure the directory exists
	err := os.MkdirAll(filepath.Dir(fullpath), os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("error creating base directory: %w", err)
	}

	f, err := os.Create(fullpath)
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(f, data)
	if err != nil {
		return nil, err
	}

	return t.Stat(path)
}
