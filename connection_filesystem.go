package artifacts

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/flanksource/artifacts/fs"
	"github.com/google/uuid"

	"github.com/flanksource/duty/connection"
	"github.com/flanksource/duty/context"
	"github.com/flanksource/duty/models"
	"github.com/flanksource/duty/types"
)

func GetFSForConnection(ctx context.Context, c models.Connection) (fs.FilesystemRW, error) {
	switch c.Type {
	case models.ConnectionTypeFolder:
		path := c.Properties["path"]
		return fs.NewLocalFS(path), nil

	case models.ConnectionTypeS3:
		var conn connection.S3Connection
		if c.ID != uuid.Nil {
			conn.ConnectionName = c.ID.String()
		} else {
			conn.Endpoint = c.URL
			conn.AccessKey = types.EnvVar{ValueStatic: c.Username}
			conn.SecretKey = types.EnvVar{ValueStatic: c.Password}
			conn.SessionToken = types.EnvVar{ValueStatic: c.Username}
			conn.Bucket = c.Properties["bucket"]
			conn.Region = c.Properties["region"]
			if val, ok := c.Properties["usePathStyle"]; ok {
				if b, err := strconv.ParseBool(val); err == nil {
					conn.UsePathStyle = b
				}
			}
			if objectPath, ok := c.Properties["objectPath"]; ok {
				conn.ObjectPath = objectPath
			}
		}

		if err := conn.Populate(ctx); err != nil {
			return nil, err
		}

		return fs.NewS3FS(ctx, conn.Bucket, conn)

	case models.ConnectionTypeGCS:
		var conn connection.GCSConnection
		if c.ID != uuid.Nil {
			conn.ConnectionName = c.ID.String()
		} else {
			conn.Credentials = &types.EnvVar{ValueStatic: c.Certificate}
			conn.Endpoint = c.URL
			conn.Bucket = c.Properties["bucket"]
		}

		if err := conn.HydrateConnection(ctx); err != nil {
			return nil, err
		}

		client, err := fs.NewGCSFS(ctx, conn.Bucket, conn)
		if err != nil {
			return nil, err
		}
		return client, nil

	case models.ConnectionTypeSFTP:
		parsedURL, err := url.Parse(c.URL)
		if err != nil {
			return nil, err
		}

		client, err := fs.NewSSHFS(fmt.Sprintf("%s:%s", parsedURL.Host, c.Properties["port"]), c.Username, c.Password)
		if err != nil {
			return nil, err
		}
		return client, nil

	case models.ConnectionTypeSMB:
		port := c.Properties["port"]
		share := c.Properties["share"]
		return fs.NewSMBFS(c.URL, port, share, types.Authentication{
			Username: types.EnvVar{ValueStatic: c.Username},
			Password: types.EnvVar{ValueStatic: c.Password},
		})
	}

	return nil, nil
}
