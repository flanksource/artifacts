package fs

import (
	gocontext "context"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	awsUtil "github.com/flanksource/artifacts/clients/aws"
	"github.com/flanksource/commons/utils"
	"github.com/flanksource/duty/connection"
	"github.com/flanksource/duty/context"
)

// s3FS implements FilesystemRW for S3
type s3FS struct {
	*s3.Client
	Bucket string
}

func NewS3FS(ctx context.Context, bucket string, conn connection.S3Connection) (*s3FS, error) {
	cfg, err := awsUtil.NewSession(ctx, conn.AWSConnection)
	if err != nil {
		return nil, err
	}

	client := &s3FS{
		Client: s3.NewFromConfig(*cfg, func(o *s3.Options) {
			o.UsePathStyle = conn.UsePathStyle
		}),
		Bucket: strings.TrimPrefix(bucket, "s3://"),
	}

	return client, nil
}

func (t *s3FS) Close() error {
	return nil // NOOP
}

func (t *s3FS) ReadDir(name string) ([]FileInfo, error) {
	req := &s3.ListObjectsInput{
		Bucket: aws.String(t.Bucket),
		Prefix: aws.String(name),
	}
	resp, err := t.Client.ListObjects(gocontext.TODO(), req)
	if err != nil {
		return nil, err
	}

	var output []FileInfo
	for _, r := range resp.Contents {
		output = append(output, &awsUtil.S3FileInfo{Object: r})
	}

	return output, nil
}

func (t *s3FS) Stat(path string) (os.FileInfo, error) {
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
	defer results.Body.Close()

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
