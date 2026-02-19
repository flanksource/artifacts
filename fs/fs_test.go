package fs

import (
	gocontext "context"
	"errors"
	"flag"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/flanksource/duty/connection"
	"github.com/flanksource/duty/context"
	"github.com/flanksource/duty/types"
	"google.golang.org/api/googleapi"

	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

var (
	endpoint   = flag.String("endpoint", "http://localhost:9000", "s3 endpoint")
	bucket     = flag.String("bucket", "test", "bucket name")
	skipVerify = flag.Bool("skip-verify", true, "http insecure skip verify")
)

var (
	accessKeyID = envDefault("TEST_AWS_ACCESS_KEY_ID", "minioadmin")
	secretKey   = envDefault("TEST_AWS_SECRET_ACCESS_KEY", "minioadmin")
	region      = envDefault("TEST_AWS_REGION", "us-east-1")
)

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

func TestFS(t *testing.T) {
	parentCtx, cancel := gocontext.WithTimeout(gocontext.Background(), time.Minute)
	defer cancel()

	ctx := context.NewContext(parentCtx)

	for _, client := range getTestClients(t, ctx) {
		t.Run(client.name, func(t *testing.T) {
			if err := populateFS(t, ctx, client.fs); err != nil {
				t.Fatalf("%v", err)
			}

			dir := "*"
			if client.name == "gcsFS" {
				dir = ""
			}

			files, err := client.fs.ReadDir(dir)
			if err != nil {
				t.Fatalf("%v", err)
			}

			if len(files) != 5 {
				t.Fatalf("expected 5 files, got %d", len(files))
			}

			// GCS doesn't support glob yet
			if client.name == "gcsFS" {
				return
			}

			{
				file, err := client.fs.Read(ctx, "record-1.txt")
				if err != nil {
					t.Fatalf("%v", err)
				}
				defer func() { _ = file.Close() }()

				content, err := io.ReadAll(file)
				if err != nil {
					t.Fatalf("%v", err)
				}

				if string(content) != "record-1" {
					t.Errorf("expected content to be record-1, got %s", string(content))
				}
			}

			{
				files, err := client.fs.ReadDir("*.json")
				if err != nil {
					t.Fatalf("%v", err)
				}

				if len(files) != 2 {
					t.Errorf("expected 2 files, got %d", len(files))
				}
			}

			{
				files, err := client.fs.ReadDir("*.yaml")
				if err != nil {
					t.Fatalf("%v", err)
				}

				if len(files) != 1 {
					t.Errorf("expected 1 files, got %d", len(files))
				}
			}

			{
				files, err := client.fs.ReadDir("record-*")
				if err != nil {
					t.Fatalf("%v", err)
				}

				if len(files) != 2 {
					t.Errorf("expected 2 files, got %d", len(files))
				}
			}
		})
	}
}

func populateFS(t *testing.T, ctx gocontext.Context, fs FilesystemRW) error {
	t.Helper()

	testData := []struct {
		name    string
		content string
	}{
		{"first.json", `{"name": "first"}`},
		{"second.json", `{"name": "second"}`},
		{"third.yaml", "third"},
		{"record-1.txt", "record-1"},
		{"record-2.txt", "record-2"},
	}

	for _, td := range testData {
		_, err := fs.Write(ctx, td.name, strings.NewReader(td.content))
		if err != nil {
			return err
		}
	}

	return nil
}

type testData struct {
	name string
	fs   FilesystemRW
}

func getTestClients(t *testing.T, ctx context.Context) []testData {
	t.Helper()

	gcsFS, err := NewGCSFS(ctx, "test", connection.GCSConnection{
		GCPConnection: connection.GCPConnection{
			SkipTLSVerify: true,
			Endpoint:      "https://localhost:4443/storage/v1/",
		},
	})
	if err != nil {
		t.Fatalf("failed to create GCS filesystem: %v", err)
	}
	createGSCBucket(t, ctx, gcsFS.Client, "fkae-project", "test")

	sshfs, err := NewSSHFS("localhost:2222", "foo", "pass")
	if err != nil {
		t.Fatalf("%v", err)
	}

	smbFS, err := NewSMBFS("localhost", "445", "users", types.Authentication{
		Username: types.EnvVar{ValueStatic: "foo"},
		Password: types.EnvVar{ValueStatic: "pass"}})
	if err != nil {
		t.Fatalf("%v", err)
	}

	s3FS, err := NewS3FS(ctx, *bucket, connection.S3Connection{
		Bucket:       *bucket,
		UsePathStyle: true,
		AWSConnection: connection.AWSConnection{
			AccessKey:     types.EnvVar{ValueStatic: accessKeyID},
			SecretKey:     types.EnvVar{ValueStatic: secretKey},
			Region:        region,
			Endpoint:      *endpoint,
			SkipTLSVerify: *skipVerify,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	createBucket(t, s3FS.Client, *bucket)

	testClients := []testData{
		{"gcsFS", gcsFS},
		{"sshfs", sshfs},
		{"smbfs", smbFS},
		{"s3FS", s3FS},
		{"local", NewLocalFS(t.TempDir())},
	}

	return testClients
}

func createBucket(t *testing.T, cl *s3.Client, bucket string) {
	t.Helper()

	_, err := cl.CreateBucket(gocontext.Background(), &s3.CreateBucketInput{
		Bucket: &bucket,
	})
	if err != nil {
		var e *s3Types.BucketAlreadyOwnedByYou
		if errors.As(err, &e) {
			return
		}
		t.Fatal(err)
	}
}

func createGSCBucket(t *testing.T, ctx context.Context, cl *storage.Client, projectID, bucket string) {
	t.Helper()

	err := cl.Bucket(bucket).Create(ctx, projectID, nil)
	if err != nil {
		var gErr *googleapi.Error
		if errors.As(err, &gErr) {
			if gErr.Code == 409 {
				return
			}
		}

		t.Fatal(err)
	}
}

func envDefault(env, def string) string {
	if os.Getenv(env) == "" {
		return def
	}

	return os.Getenv(env)
}
