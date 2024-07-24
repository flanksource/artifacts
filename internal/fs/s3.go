package fs

import (
	gocontext "context"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/bmatcuk/doublestar/v4"
	awsUtil "github.com/flanksource/artifacts/clients/aws"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/commons/utils"
	"github.com/flanksource/duty/connection"
	"github.com/flanksource/duty/context"
	jszwecs3fs "github.com/jszwec/s3fs/v2"
	"github.com/samber/lo"
)

type s3FileInfo struct {
	key    string
	object s3.GetObjectOutput
}

func (fi s3FileInfo) Name() string {
	return filepath.Base(fi.key)
}

func (fi s3FileInfo) Size() int64 {
	return *fi.object.ContentLength
}

func (fi s3FileInfo) Mode() fs.FileMode {
	return 0644 // TODO:
}

func (fi s3FileInfo) ModTime() time.Time {
	return lo.FromPtr(fi.object.LastModified)
}

func (fi s3FileInfo) IsDir() bool {
	return false // TODO:
}

func (fi s3FileInfo) Sys() any {
	return nil
}

type s3File struct {
	key    string
	object s3.GetObjectOutput
}

func (t *s3File) Stat() (fs.FileInfo, error) {
	return s3FileInfo{object: t.object, key: t.key}, nil
}

func (t *s3File) Close() error {
	return t.object.Body.Close()
}

func (t *s3File) Read(b []byte) (int, error) {
	return t.object.Body.Read(b)
}

type s3DirFS struct {
	*s3FS
	base string
}

func (t *s3DirFS) Open(name string) (fs.File, error) {
	output, err := t.Client.GetObject(gocontext.TODO(), &s3.GetObjectInput{
		Bucket: &t.Bucket,
		Key:    &name,
	})
	if err != nil {
		return nil, err
	}

	return &s3File{object: *output, key: name}, nil
}

func (t *s3DirFS) Stat(path string) (fs.FileInfo, error) {
	return t.s3FS.Stat(filepath.Join(t.base, path))
}

func (t *s3DirFS) ReadDir(name string) ([]fs.DirEntry, error) {
	prefix := path.Join(t.base, name)
	if prefix == "." {
		prefix = ""
	}

	input := &s3.ListObjectsV2Input{
		Bucket: &t.Bucket,
		Prefix: &prefix,
	}

	var entries []fs.DirEntry
	paginator := s3.NewListObjectsV2Paginator(t.Client, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(gocontext.TODO())
		if err != nil {
			return nil, err
		}

		for _, object := range page.Contents {
			key := *object.Key
			if key == prefix {
				continue // Skip the directory itself
			}
			entry := s3DirEntry{
				key:          key,
				size:         *object.Size,
				lastModified: *object.LastModified,
			}
			entries = append(entries, &entry)
		}
	}

	return entries, nil
}

type s3DirEntry struct {
	key          string
	size         int64
	lastModified time.Time
}

func (e *s3DirEntry) Name() string {
	return filepath.Base(e.key)
}

func (e *s3DirEntry) IsDir() bool {
	return strings.HasSuffix(e.key, "/")
}

func (e *s3DirEntry) Type() fs.FileMode {
	if e.IsDir() {
		return fs.ModeDir
	}
	return 0
}

func (e *s3DirEntry) Info() (fs.FileInfo, error) {
	return s3FileInfo{
		key: e.key,
		object: s3.GetObjectOutput{
			ContentLength: &e.size,
			LastModified:  &e.lastModified,
		},
	}, nil
}

// s3FS implements
// - FilesystemRW for S3
// - fs.FS for glob support
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

func (t *s3FS) ReadDir(pattern string) ([]FileInfo, error) {
	base, pattern := doublestar.SplitPattern(pattern)
	matches, err := doublestar.Glob(jszwecs3fs.New(t.Client, t.Bucket).WithBase(base), pattern)
	if err != nil {
		return nil, err
	}

	logger.Infof("matches: %d", len(matches))

	dir := filepath.Dir(pattern)
	if dir == "." || dir == "**" {
		dir = ""
	}

	req := &s3.ListObjectsInput{
		Bucket: aws.String(t.Bucket),
		Prefix: aws.String(dir),
	}

	resp, err := t.Client.ListObjects(gocontext.TODO(), req)
	if err != nil {
		return nil, err
	}

	var output []FileInfo
	for _, r := range resp.Contents {
		name := strings.TrimPrefix(*r.Key, dir)
		if dir != "" {
			name = strings.TrimPrefix(name, "/")
		}

		match, err := filepath.Match(filepath.Base(pattern), name)
		if err != nil {
			return nil, err
		}

		if match {
			output = append(output, &awsUtil.S3FileInfo{Object: r})
		}
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
