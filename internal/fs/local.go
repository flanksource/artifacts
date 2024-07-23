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

type localFileInfo struct {
	os.FileInfo
	path string
}

func (t localFileInfo) FullPath() string {
	return t.path
}

func NewLocalFS(base string) *localFS {
	return &localFS{base: base}
}

func (t *localFS) Close() error {
	return nil
}

func (t *localFS) ReadDir(name string) ([]FileInfo, error) {
	pattern := filepath.Join(t.base, name)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	output := make([]FileInfo, 0, len(matches))
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			return nil, err
		}

		output = append(output, localFileInfo{FileInfo: info, path: match})
	}

	return output, nil
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
