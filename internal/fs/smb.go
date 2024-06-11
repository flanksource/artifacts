package fs

import (
	gocontext "context"
	"fmt"
	"io"
	"os"

	"github.com/flanksource/artifacts/clients/smb"
	"github.com/flanksource/duty/types"
)

type smbFS struct {
	*smb.SMBSession
}

func NewSMBFS(server string, port, share string, auth types.Authentication) (*smbFS, error) {
	session, err := smb.SMBConnect(server, port, share, auth)
	if err != nil {
		return nil, err
	}

	return &smbFS{SMBSession: session}, nil
}

func (s *smbFS) Read(ctx gocontext.Context, path string) (io.ReadCloser, error) {
	return s.SMBSession.Share.Open(path)
}

func (s *smbFS) Write(ctx gocontext.Context, path string, data io.Reader) (os.FileInfo, error) {
	f, err := s.SMBSession.Share.Create(path)
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(f, data)
	if err != nil {
		return nil, fmt.Errorf("error writing file: %w", err)
	}

	return f.Stat()
}
