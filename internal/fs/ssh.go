package fs

import (
	gocontext "context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	sftpClient "github.com/flanksource/artifacts/clients/sftp"
	"github.com/pkg/sftp"
)

type sshFS struct {
	*sftp.Client
}

func NewSSHFS(host, user, password string) (*sshFS, error) {
	sftpClient, err := sftpClient.SSHConnect(host, user, password)
	if err != nil {
		return nil, err
	}

	return &sshFS{
		Client: sftpClient,
	}, nil
}

func (s *sshFS) Read(ctx gocontext.Context, fileID string) (io.ReadCloser, error) {
	return s.Client.Open(fileID)
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
