package artifacts

import (
	gocontext "context"
	"io"
	"net/url"
	"os"

	"github.com/flanksource/artifacts/internal/fs"

	"github.com/flanksource/duty/connection"
	"github.com/flanksource/duty/context"
	"github.com/flanksource/duty/models"
	"github.com/flanksource/duty/types"
)

type Filesystem interface {
	Close() error
	ReadDir(name string) ([]os.FileInfo, error)
	Stat(name string) (os.FileInfo, error)
}

type FilesystemRW interface {
	Filesystem
	Read(ctx gocontext.Context, fileID string) (io.ReadCloser, error)
	Write(ctx gocontext.Context, path string, data io.Reader) (os.FileInfo, error)
}

func GetFSForConnection(ctx context.Context, c models.Connection) (FilesystemRW, error) {
	switch c.Type {
	case models.ConnectionTypeFolder:
		path := c.Properties["path"]
		return fs.NewLocalFS(path), nil

	case models.ConnectionTypeAWS:
		bucket := c.Properties["bucket"]
		conn := connection.AWSConnection{
			ConnectionName: c.ID.String(),
		}
		if err := conn.Populate(ctx); err != nil {
			return nil, err
		}

		return fs.NewS3FS(ctx, bucket, conn)

	case models.ConnectionTypeGCP:
		bucket := c.Properties["bucket"]
		conn := &connection.GCPConnection{
			ConnectionName: c.ID.String(),
		}
		if err := conn.HydrateConnection(ctx); err != nil {
			return nil, err
		}

		client, err := fs.NewGCSFS(ctx, bucket, conn)
		if err != nil {
			return nil, err
		}
		return client, nil

	case models.ConnectionTypeSFTP:
		parsedURL, err := url.Parse(c.URL)
		if err != nil {
			return nil, err
		}

		client, err := fs.NewSSHFS(parsedURL.Host, c.Username, c.Password)
		if err != nil {
			return nil, err
		}
		return client, nil

	case models.ConnectionTypeSMB:
		port := c.Properties["port"]
		share := c.Properties["share"]
		return fs.NewSMBFS(c.URL, port, share, connection.Authentication{
			Username: types.EnvVar{ValueStatic: c.Username},
			Password: types.EnvVar{ValueStatic: c.Password},
		})
	}

	return nil, nil
}
