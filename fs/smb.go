package fs

import (
	gocontext "context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/flanksource/artifacts/clients/smb"
	"github.com/flanksource/duty/types"
)

type smbFS struct {
	*smb.SMBSession
}

type SMBFileInfo struct {
	Base string
	fs.FileInfo
}

func (t *SMBFileInfo) FullPath() string {
	return path.Join(t.Base, t.FileInfo.Name())
}

func NewSMBFS(server string, port, share string, auth types.Authentication) (*smbFS, error) {
	if port == "" {
		port = "445"
	}

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

func (t *smbFS) ReadDir(name string) ([]FileInfo, error) {
	if strings.Contains(name, "*") {
		return t.ReadDirGlob(name)
	}

	fileInfos, err := t.SMBSession.ReadDir(name)
	if err != nil {
		return nil, err
	}

	output := make([]FileInfo, 0, len(fileInfos))
	for _, fileInfo := range fileInfos {
		output = append(output, &SMBFileInfo{Base: name, FileInfo: fileInfo})
	}

	return output, nil
}

func (t *smbFS) ReadDirGlob(name string) ([]FileInfo, error) {
	base, pattern := doublestar.SplitPattern(name)
	matches, err := doublestar.Glob(t.DirFS(base), pattern)
	if err != nil {
		return nil, fmt.Errorf("error globbing pattern %q: %w", pattern, err)
	}

	output := make([]FileInfo, 0, len(matches))
	for _, match := range matches {
		fullPath := filepath.Join(base, match)
		info, err := t.Stat(fullPath)
		if err != nil {
			return nil, err
		}

		output = append(output, &SMBFileInfo{Base: name, FileInfo: info})
	}

	return output, nil
}
