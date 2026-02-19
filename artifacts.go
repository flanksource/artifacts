package artifacts

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"

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

type Artifact struct {
	ContentType string
	Path        string
	Content     io.ReadCloser
}

const maxBytesForMimeDetection = 512 * 1024 // 512KB

func SaveArtifact(ctx context.Context, fs artifactFS.FilesystemRW, artifact *models.Artifact, data Artifact) error {
	defer func() { _ = data.Content.Close() }()

	checksum := sha256.New()
	mimeReader := io.TeeReader(data.Content, checksum)

	mimeWriter := &MIMEWriter{Max: maxBytesForMimeDetection}
	fileReader := io.TeeReader(mimeReader, mimeWriter)

	info, err := fs.Write(ctx, data.Path, fileReader)
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
