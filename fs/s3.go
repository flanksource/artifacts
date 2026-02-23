package fs

import (
	"bytes"
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
	"github.com/samber/lo"
)

const s3ListObjectMaxKeys = 1000

// s3FS implements
// - FilesystemRW for S3
// - fs.FS for glob support
type s3FS struct {
	// maxObjects limits the total number of objects ReadDir can return.
	maxObjects int

	Client *s3.Client
	Bucket string
}

func NewS3FS(ctx context.Context, bucket string, conn connection.S3Connection) (*s3FS, error) {
	if err := conn.Populate(ctx); err != nil {
		return nil, err
	}

	cfg, err := conn.Client(ctx)
	if err != nil {
		return nil, err
	}

	client := &s3FS{
		maxObjects: 50 * 10_000,
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

func (t *s3FS) SetMaxListItems(max int) {
	t.maxObjects = max
}

func (t *s3FS) Close() error {
	return nil // NOOP
}

func (t *s3FS) ReadDir(pattern string) ([]FileInfo, error) {
	prefix, glob := doublestar.SplitPattern(pattern)
	if prefix == "." {
		prefix = ""
	}

	req := &s3.ListObjectsV2Input{
		Bucket: aws.String(t.Bucket),
		Prefix: aws.String(prefix),
	}

	if t.maxObjects < s3ListObjectMaxKeys {
		req.MaxKeys = lo.ToPtr(int32(t.maxObjects))
	}

	hasGlob := glob != ""
	var output []FileInfo
	var numObjectsFetched int
	for {
		resp, err := t.Client.ListObjectsV2(gocontext.TODO(), req)
		if err != nil {
			return nil, err
		}

		for _, obj := range resp.Contents {
			if hasGlob {
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

		numObjectsFetched += int(*resp.KeyCount)
		if numObjectsFetched >= t.maxObjects {
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
	// Try to determine content length from the reader using type-based heuristics
	contentLength := getContentLength(data)

	var body io.Reader
	if contentLength >= 0 {
		// Content length is known, use the reader directly
		body = data
	} else {
		// Content length unknown, need to buffer to determine size
		// This is required because S3 PutObject requires Content-Length header
		content, err := io.ReadAll(data)
		if err != nil {
			return nil, err
		}
		contentLength = int64(len(content))
		body = bytes.NewReader(content)
	}

	_, err := t.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(t.Bucket),
		Key:           aws.String(path),
		Body:          body,
		ContentLength: &contentLength,
	})

	if err != nil {
		return nil, err
	}

	return t.Stat(path)
}

// getContentLength attempts to determine content length from the reader using heuristics
func getContentLength(r io.Reader) int64 {
	// Check for our custom readerWithLength wrapper
	if rwl, ok := r.(interface{ ContentLength() int64 }); ok {
		return rwl.ContentLength()
	}

	// Try common interfaces that provide size information
	switch v := r.(type) {
	case interface{ Len() int }:
		return int64(v.Len())
	case interface{ Size() int64 }:
		return v.Size()
	case *bytes.Reader:
		return int64(v.Len())
	case *strings.Reader:
		return int64(v.Len())
	case *os.File:
		if stat, err := v.Stat(); err == nil {
			return stat.Size()
		}
	}

	// Check if it's a seeker (but don't modify position for streaming readers)
	if seeker, ok := r.(io.Seeker); ok {
		// Only try this for known seekable types
		if _, isBytesReader := r.(*bytes.Reader); isBytesReader {
			current, err := seeker.Seek(0, io.SeekCurrent)
			if err == nil {
				end, err := seeker.Seek(0, io.SeekEnd)
				if err == nil {
					_, _ = seeker.Seek(current, io.SeekStart)
					return end - current
				}
			}
		}
	}

	// Unknown length
	return -1
}
