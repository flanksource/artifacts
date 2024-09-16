package fs

import (
	gocontext "context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	sftpClient "github.com/flanksource/artifacts/clients/sftp"
	"github.com/pkg/sftp"
)

type sshFS struct {
	*sftp.Client
	wd string
}

type sshFileInfo struct {
	fullpath string
	fs.FileInfo
}

func (t *sshFileInfo) FullPath() string {
	return t.fullpath
}

func NewSSHFS(host, user, password string) (*sshFS, error) {
	sftpClient, err := sftpClient.SSHConnect(host, user, password)
	if err != nil {
		return nil, err
	}

	wd, err := sftpClient.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	return &sshFS{
		wd:     wd,
		Client: sftpClient,
	}, nil
}

func (t *sshFS) ReadDir(name string) ([]FileInfo, error) {
	if strings.Contains(name, "*") {
		return t.ReadDirGlob(name)
	}

	files, err := t.Client.ReadDir(name)
	if err != nil {
		return nil, err
	}

	output := make([]FileInfo, 0, len(files))
	for _, file := range files {
		base := name
		if !strings.HasPrefix(name, "/") {
			base = filepath.Join(t.wd, name)
		}

		output = append(output, &sshFileInfo{FileInfo: file, fullpath: filepath.Join(base, file.Name())})
	}

	return output, nil
}

func (t *sshFS) ReadDirGlob(name string) ([]FileInfo, error) {
	// TODO: This doesn't fully support doublestar
	entries, err := t.Client.Glob(name)
	if err != nil {
		return nil, err
	}

	output := make([]FileInfo, 0, len(entries))
	for _, entry := range entries {
		info, err := t.Stat(entry)
		if err != nil {
			return nil, err
		}

		output = append(output, &sshFileInfo{FileInfo: info, fullpath: entry})
	}

	return output, nil
}

func (s *sshFS) Read(ctx gocontext.Context, path string) (io.ReadCloser, error) {
	return s.Client.Open(path)
}

func (s *sshFS) Write(ctx gocontext.Context, path string, data io.Reader) (os.FileInfo, error) {
	// Ensure the directory exists
	dir := filepath.Dir(path)
	err := s.Client.MkdirAll(dir)
	if err != nil {
		return nil, fmt.Errorf("error creating directory: %w", err)
	}

	f, err := s.Client.Create(path)
	if err != nil {
		return nil, fmt.Errorf("error creating file: %w", err)
	}

	_, err = io.Copy(f, data)
	if err != nil {
		return nil, fmt.Errorf("error writing to file: %w", err)
	}

	return f.Stat()
}
