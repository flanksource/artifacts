package fs_test

import (
	gocontext "context"
	"errors"
	"flag"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/flanksource/artifacts/fs"
	"github.com/flanksource/duty/connection"
	"github.com/flanksource/duty/context"
	"github.com/flanksource/duty/types"
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

func TestS3Glob(t *testing.T) {
	ctx := context.NewContext(gocontext.Background())
	fs, err := fs.NewS3FS(ctx, *bucket, connection.S3Connection{
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

	// json/
	// └── flanksource/
	//     ├── users/
	//     │   ├── aditya.json
	//     │   └── yash.json
	//     ├── projects/
	//     │   ├── mission-control.json
	//     │   └── canary-checker.json
	//     └── tech/
	//         └── cloud/
	//             ├── aws.json
	//             └── azure.json

	createBucket(t, fs.Client, *bucket)
	writeFile(t, fs.Client, *bucket, "json/flanksource/users/aditya.json", []byte(`{"name": "Aditya"}`))
	writeFile(t, fs.Client, *bucket, "json/flanksource/users/yash.json", []byte(`{"name": "Yash"}`))
	writeFile(t, fs.Client, *bucket, "json/flanksource/projects/mission-control.json", []byte(`{"name": "Mission Control"}`))
	writeFile(t, fs.Client, *bucket, "json/flanksource/projects/canary-checker.json", []byte(`{"name": "Canary Checker"}`))
	writeFile(t, fs.Client, *bucket, "json/flanksource/tech/cloud/aws.json", []byte(`{"name": "AWS"}`))
	writeFile(t, fs.Client, *bucket, "json/flanksource/tech/cloud/azure.json", []byte(`{"name": "Azure"}`))

	testData := []struct {
		pattern    string
		count      int
		maxObjects int
	}{
		{
			pattern: "json/**/*.json",
			count:   6,
		},
		{
			pattern: "json/**/users/*.json",
			count:   2,
		},
		{
			pattern: "json/**/tech/cloud/*.json",
			count:   2,
		},
		{
			pattern: "**/*.yaml",
			count:   0,
		},
		{
			pattern: "**/*.json",
			count:   6,
		},
		{
			pattern:    "**/*.json",
			count:      2,
			maxObjects: 2,
		},
	}

	for _, td := range testData {
		if td.maxObjects != 0 {
			fs.SetMaxListItems(td.maxObjects)
		}

		list, err := fs.ReadDir(td.pattern)
		if err != nil {
			t.Fatal(err)
		}

		if len(list) != td.count {
			t.Fatalf("expected %d files, got %d", td.count, len(list))
		}
	}
}

func envDefault(env, def string) string {
	if os.Getenv(env) == "" {
		return def
	}

	return os.Getenv(env)
}

func writeFile(t *testing.T, cl *s3.Client, bucket, name string, data []byte) {
	t.Helper()

	uploader := manager.NewUploader(cl)
	_, err := uploader.Upload(gocontext.Background(), &s3.PutObjectInput{
		Body:   strings.NewReader(string(data)),
		Bucket: &bucket,
		Key:    &name,
	})
	if err != nil {
		t.Fatal(err)
	}
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
