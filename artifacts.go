package artifacts

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	artifactFS "github.com/flanksource/artifacts/fs"
	"github.com/flanksource/duty/context"
	"github.com/flanksource/duty/models"
	"github.com/gabriel-vasile/mimetype"
)

// MIMEWriter implements io.Writer with a limit on the number of bytes used for detection.
type MIMEWriter struct {
	buffer []byte

	Max int // max number of bytes to use from the source
}

func (t *MIMEWriter) Write(bb []byte) (n int, err error) {
	if len(t.buffer) > t.Max {
		return 0, nil
	}

	rem := t.Max - len(t.buffer)
	if rem > len(bb) {
		rem = len(bb)
	}
	t.buffer = append(t.buffer, bb[:rem]...)
	return rem, nil
}

func (t *MIMEWriter) Detect() *mimetype.MIME {
	return mimetype.Detect(t.buffer)
}

// readerWithLength wraps an io.Reader and carries content length information
type readerWithLength struct {
	reader io.Reader
	length int64
}

func (r *readerWithLength) Read(p []byte) (n int, err error) {
	return r.reader.Read(p)
}

// ContentLength returns the content length if known, -1 otherwise
func (r *readerWithLength) ContentLength() int64 {
	return r.length
}

// determineContentLength attempts to determine the content length from the reader
// using type assertions and heuristics
func determineContentLength(r io.Reader) int64 {
	// Try common interfaces that provide size information
	switch v := r.(type) {
	case interface{ Len() int }:
		return int64(v.Len())
	case interface{ Size() int64 }:
		return v.Size()
	case *os.File:
		if stat, err := v.Stat(); err == nil {
			return stat.Size()
		}
	}

	// Check if it's a bytes.Reader or strings.Reader
	if seeker, ok := r.(io.Seeker); ok {
		// Get current position
		current, err := seeker.Seek(0, io.SeekCurrent)
		if err == nil {
			// Seek to end to get size
			end, err := seeker.Seek(0, io.SeekEnd)
			if err == nil {
				// Restore original position
				_, _ = seeker.Seek(current, io.SeekStart)
				return end - current
			}
		}
	}

	// Unknown length
	return -1
}

type Artifact struct {
	ContentType   string
	Path          string
	Content       io.ReadCloser
	ContentLength int64 // Optional: content length if known, -1 if unknown
}

const maxBytesForMimeDetection = 512 * 1024 // 512KB

func SaveArtifact(ctx context.Context, fs artifactFS.FilesystemRW, artifact *models.Artifact, data Artifact) error {
	defer func() { _ = data.Content.Close() }()

	// Determine content length if not already provided
	if data.ContentLength < 0 {
		data.ContentLength = determineContentLength(data.Content)
	}

	checksum := sha256.New()
	mimeReader := io.TeeReader(data.Content, checksum)

	mimeWriter := &MIMEWriter{Max: maxBytesForMimeDetection}
	fileReader := io.TeeReader(mimeReader, mimeWriter)

	// Create a reader wrapper that carries the content length
	wrappedReader := &readerWithLength{
		reader: fileReader,
		length: data.ContentLength,
	}

	info, err := fs.Write(ctx, data.Path, wrappedReader)
	if err != nil {
		return fmt.Errorf("error writing artifact(%s): %w", data.Path, err)
	}

	if data.ContentType == "" {
		data.ContentType = mimeWriter.Detect().String()
	}

	artifact.Path = data.Path
	artifact.Filename = info.Name()
	artifact.Size = info.Size()
	artifact.ContentType = data.ContentType
	artifact.Checksum = hex.EncodeToString(checksum.Sum(nil))
	if err := ctx.DB().Create(&artifact).Error; err != nil {
		return fmt.Errorf("error saving artifact to db: %w", err)
	}

	return nil
}
