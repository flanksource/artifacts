package fs

import (
	gocontext "context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/bmatcuk/doublestar/v4"
	awsUtil "github.com/flanksource/artifacts/clients/aws"
	"github.com/flanksource/commons/utils"
	"github.com/flanksource/duty/connection"
	"github.com/flanksource/duty/context"
	"github.com/flanksource/s3fs/v2"
)

// s3FS implements
// - FilesystemRW for S3
// - fs.FS for glob support
type s3FS struct {
	Client *s3.Client
	Bucket string
}

func NewS3FS(ctx context.Context, bucket string, conn connection.S3Connection) (*s3FS, error) {
	if err := conn.Populate(ctx); err != nil {
		return nil, err
	}

	cfg, err := conn.AWSConnection.Client(ctx)
	if err != nil {
		return nil, err
	}

	client := &s3FS{
		Client: s3.NewFromConfig(cfg, func(o *s3.Options) {
			o.UsePathStyle = conn.UsePathStyle

			if conn.Endpoint != "" {
				o.BaseEndpoint = &conn.Endpoint
			}
		}),
		Bucket: strings.TrimPrefix(bucket, "s3://"),
	}

	return client, nil
}

func (t *s3FS) Close() error {
	return nil // NOOP
}

func (t *s3FS) ReadDir(pattern string) ([]FileInfo, error) {
	base, pattern := doublestar.SplitPattern(pattern)
	matches, err := doublestar.Glob(s3fs.New(t.Client, t.Bucket).WithPrefix(base), pattern)
	if err != nil {
		return nil, err
	}

	var output []FileInfo
	for _, r := range matches {
		fullpath := filepath.Join(base, r)
		output = append(output, &awsUtil.S3FileInfo{Object: s3Types.Object{Key: &fullpath}})
	}

	return output, nil
}

func (t *s3FS) Stat(path string) (fs.FileInfo, error) {
	headObject, err := t.Client.HeadObject(gocontext.TODO(), &s3.HeadObjectInput{
		Bucket: aws.String(t.Bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return nil, err
	}

	fileInfo := &awsUtil.S3FileInfo{
		Object: s3Types.Object{
			Key:          utils.Ptr(filepath.Base(path)),
			Size:         headObject.ContentLength,
			LastModified: headObject.LastModified,
			ETag:         headObject.ETag,
		},
	}

	return fileInfo, nil
}

func (t *s3FS) Read(ctx gocontext.Context, key string) (io.ReadCloser, error) {
	results, err := t.Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(t.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}

	return results.Body, nil
}

func (t *s3FS) Write(ctx gocontext.Context, path string, data io.Reader) (os.FileInfo, error) {
	_, err := t.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(t.Bucket),
		Key:    aws.String(path),
		Body:   data,
	})

	if err != nil {
		return nil, err
	}

	return t.Stat(path)
}
