package fs

// import (
// 	"io/fs"
// 	"path/filepath"

// 	"github.com/pkg/sftp"
// )

// type sftpDirFS struct {
// 	client *sftp.Client
// 	fs.File
// 	path string
// }

// var _ fs.FS = (*sftpDirFS)(nil)
// var _ fs.ReadDirFile = (*sftpDirFS)(nil)

// func (f *sftpDirFS) Open(name string) (fs.File, error) {
// 	ff, err := f.client.Open(name)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return &sftpDirFS{client: f.client, File: ff, path: filepath.Join(f.path, ff.Name())}, nil
// }

// func (f *sftpDirFS) ReadDir(n int) ([]fs.DirEntry, error) {
// 	entries, err := f.client.ReadDir(f.path)
// 	if err != nil {
// 		return nil, err
// 	}

// 	var output []fs.DirEntry
// 	for _, entry := range entries {
// 		output = append(output, &dirEntry{FileInfo: entry})
// 	}

// 	return output, nil
// }

// var _ fs.DirEntry = (*dirEntry)(nil)

// type dirEntry struct {
// 	fs.FileInfo
// }

// func (t *dirEntry) Info() (fs.FileInfo, error) {
// 	return t.FileInfo, nil
// }

// func (t *dirEntry) IsDir() bool {
// 	return t.FileInfo.IsDir()
// }

// func (t *dirEntry) Type() fs.FileMode {
// 	return t.FileInfo.Mode()
// }

// func (t *dirEntry) Name() string {
// 	return t.FileInfo.Name()
// }
