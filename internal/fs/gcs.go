package fs

import (
	gocontext "context"
	"io"
	"os"
	"strings"

	gcs "cloud.google.com/go/storage"
	gcpUtil "github.com/flanksource/artifacts/clients/gcp"
	"github.com/flanksource/duty/connection"
	"github.com/flanksource/duty/context"
)

// gcsFS implements FilesystemRW for Google Cloud Storage
type gcsFS struct {
	*gcs.Client
	Bucket string
}

func NewGCSFS(ctx context.Context, bucket string, conn *connection.GCPConnection) (*gcsFS, error) {
	cfg, err := gcpUtil.NewSession(ctx, conn)
	if err != nil {
		return nil, err
	}

	client := gcsFS{
		Bucket: strings.TrimPrefix(bucket, "gcs://"),
		Client: cfg,
	}

	return &client, nil
}

func (t *gcsFS) Close() error {
	return t.Client.Close()
}

// TODO: implement
func (t *gcsFS) ReadDir(name string) ([]os.FileInfo, error) {
	return nil, nil
}

func (t *gcsFS) Stat(path string) (os.FileInfo, error) {
	obj := t.Client.Bucket(t.Bucket).Object(path)
	attrs, err := obj.Attrs(gocontext.TODO())
	if err != nil {
		return nil, err
	}

	fileInfo := &gcpUtil.GCSFileInfo{
		Object: attrs,
	}

	return fileInfo, nil
}

func (t *gcsFS) Read(ctx gocontext.Context, fileID string) (io.ReadCloser, error) {
	obj := t.Client.Bucket(t.Bucket).Object(fileID)

	reader, err := obj.NewReader(ctx)
	if err != nil {
		return nil, err
	}

	return reader, nil
}

func (t *gcsFS) Write(ctx gocontext.Context, path string, data io.Reader) (os.FileInfo, error) {
	obj := t.Client.Bucket(t.Bucket).Object(path)

	content, err := io.ReadAll(data)
	if err != nil {
		return nil, err
	}

	writer := obj.NewWriter(ctx)
	if _, err := writer.Write(content); err != nil {
		return nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	return t.Stat(path)
}
