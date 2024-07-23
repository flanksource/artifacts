package fs

import "os"

// FileInfo is a wrapper for os.FileInfo that also returns the full path of the file.
//
// FullPath is required to support globs with ReadDir()
type FileInfo interface {
	os.FileInfo
	FullPath() string
}
