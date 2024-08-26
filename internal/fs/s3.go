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
)

// s3FS implements
// - FilesystemRW for S3
// - fs.FS for glob support
type s3FS struct {
	maxList *int32
	Client  *s3.Client
	Bucket  string
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

func (t *s3FS) SetMaxList(maxList int32) {
	t.maxList = &maxList
}

func (t *s3FS) Close() error {
	return nil // NOOP
}

func (t *s3FS) ReadDir(pattern string) ([]FileInfo, error) {
	prefix, _ := doublestar.SplitPattern(pattern)
	if prefix == "." {
		prefix = ""
	}

	req := &s3.ListObjectsV2Input{
		Bucket:  aws.String(t.Bucket),
		Prefix:  aws.String(prefix),
		MaxKeys: t.maxList,
	}

	var output []FileInfo
	for {
		resp, err := t.Client.ListObjectsV2(gocontext.TODO(), req)
		if err != nil {
			return nil, err
		}

		for _, obj := range resp.Contents {
			if pattern != "" {
				if matched, err := doublestar.Match(pattern, *obj.Key); err != nil {
					return nil, err
				} else if !matched {
					continue
				}
			}

			fileInfo := &awsUtil.S3FileInfo{Object: obj}
			output = append(output, fileInfo)
		}

		if resp.NextContinuationToken == nil {
			break
		}

		req.ContinuationToken = resp.NextContinuationToken
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
