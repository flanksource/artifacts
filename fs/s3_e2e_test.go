package fs

import (
	gocontext "context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/flanksource/duty/connection"
	"github.com/flanksource/duty/context"
	"github.com/flanksource/duty/types"
)

// TestS3E2E_ContentLength tests the S3 Content-Length fix with LocalStack
// This test validates that PutObject works correctly with Content-Length header
// when using io.Reader chains (TeeReaders) as used in the artifact system
func TestS3E2E_ContentLength(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	parentCtx, cancel := gocontext.WithTimeout(gocontext.Background(), 2*time.Minute)
	defer cancel()

	ctx := context.NewContext(parentCtx)

	// Test with LocalStack
	t.Run("LocalStack", func(t *testing.T) {
		testS3ContentLength(t, ctx, "http://localhost:4566", "test", "us-east-1")
	})

	// Test with MinIO as well for compatibility
	t.Run("MinIO", func(t *testing.T) {
		testS3ContentLength(t, ctx, "http://localhost:9000", "test", "us-east-1")
	})
}

func testS3ContentLength(t *testing.T, ctx context.Context, endpoint, bucket, region string) {
	t.Helper()

	// Create S3 filesystem client
	s3FS, err := NewS3FS(ctx, bucket, connection.S3Connection{
		Bucket:       bucket,
		UsePathStyle: true,
		AWSConnection: connection.AWSConnection{
			AccessKey:     types.EnvVar{ValueStatic: "test"},
			SecretKey:     types.EnvVar{ValueStatic: "test"},
			Region:        region,
			Endpoint:      endpoint,
			SkipTLSVerify: true,
		},
	})
	if err != nil {
		t.Fatalf("failed to create S3 filesystem: %v", err)
	}
	defer s3FS.Close()

	// Create bucket if it doesn't exist
	createBucket(t, s3FS.Client, bucket)

	// Test 1: Simple write with small content
	t.Run("SimpleWrite", func(t *testing.T) {
		content := "Hello, World!"
		key := "test-simple.txt"

		info, err := s3FS.Write(ctx, key, strings.NewReader(content))
		if err != nil {
			t.Fatalf("failed to write to S3: %v", err)
		}

		if info.Size() != int64(len(content)) {
			t.Errorf("expected size %d, got %d", len(content), info.Size())
		}

		// Verify we can read it back
		reader, err := s3FS.Read(ctx, key)
		if err != nil {
			t.Fatalf("failed to read from S3: %v", err)
		}
		defer reader.Close()

		readContent, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("failed to read content: %v", err)
		}

		if string(readContent) != content {
			t.Errorf("expected content %q, got %q", content, string(readContent))
		}
	})

	// Test 2: Write with TeeReader chain (simulates artifact saving with checksum)
	// This is the critical test case that was failing before the fix
	t.Run("WriteWithTeeReader", func(t *testing.T) {
		content := `{"status": "success", "data": [1,2,3,4,5], "message": "test artifact"}`
		key := "test-artifact.json"

		// Create a TeeReader chain similar to what SaveArtifact does
		checksum := sha256.New()
		teeReader := io.TeeReader(strings.NewReader(content), checksum)

		info, err := s3FS.Write(ctx, key, teeReader)
		if err != nil {
			t.Fatalf("failed to write with TeeReader to S3: %v", err)
		}

		if info.Size() != int64(len(content)) {
			t.Errorf("expected size %d, got %d", len(content), info.Size())
		}

		// Verify checksum was calculated
		expectedChecksum := sha256.Sum256([]byte(content))
		actualChecksum := hex.EncodeToString(checksum.Sum(nil))
		expectedChecksumStr := hex.EncodeToString(expectedChecksum[:])

		if actualChecksum != expectedChecksumStr {
			t.Errorf("checksum mismatch: expected %s, got %s", expectedChecksumStr, actualChecksum)
		}

		// Verify we can read it back
		reader, err := s3FS.Read(ctx, key)
		if err != nil {
			t.Fatalf("failed to read from S3: %v", err)
		}
		defer reader.Close()

		readContent, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("failed to read content: %v", err)
		}

		if string(readContent) != content {
			t.Errorf("expected content %q, got %q", content, string(readContent))
		}
	})

	// Test 3: Write with multiple TeeReaders (most complex case)
	t.Run("WriteWithMultipleTeeReaders", func(t *testing.T) {
		content := strings.Repeat("abcdefghijklmnopqrstuvwxyz", 100) // ~2.6KB
		key := "test-large-artifact.txt"

		// Create multiple TeeReaders to simulate checksum + MIME detection
		checksum := sha256.New()
		teeReader1 := io.TeeReader(strings.NewReader(content), checksum)

		// Second TeeReader for MIME detection simulation
		var mimeBuffer []byte
		mimeWriter := &testWriter{buffer: &mimeBuffer, max: 512}
		teeReader2 := io.TeeReader(teeReader1, mimeWriter)

		info, err := s3FS.Write(ctx, key, teeReader2)
		if err != nil {
			t.Fatalf("failed to write with multiple TeeReaders to S3: %v", err)
		}

		if info.Size() != int64(len(content)) {
			t.Errorf("expected size %d, got %d", len(content), info.Size())
		}

		// Verify checksum
		expectedChecksum := sha256.Sum256([]byte(content))
		actualChecksum := hex.EncodeToString(checksum.Sum(nil))
		expectedChecksumStr := hex.EncodeToString(expectedChecksum[:])

		if actualChecksum != expectedChecksumStr {
			t.Errorf("checksum mismatch: expected %s, got %s", expectedChecksumStr, actualChecksum)
		}

		// Verify we can read it back
		reader, err := s3FS.Read(ctx, key)
		if err != nil {
			t.Fatalf("failed to read from S3: %v", err)
		}
		defer reader.Close()

		readContent, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("failed to read content: %v", err)
		}

		if string(readContent) != content {
			t.Errorf("content mismatch: lengths differ (expected %d, got %d)", len(content), len(readContent))
		}
	})

	// Test 4: Large file (tests that buffering works with larger payloads)
	t.Run("LargeFile", func(t *testing.T) {
		// Create a 1MB file
		content := strings.Repeat("x", 1024*1024)
		key := "test-large-file.bin"

		info, err := s3FS.Write(ctx, key, strings.NewReader(content))
		if err != nil {
			t.Fatalf("failed to write large file to S3: %v", err)
		}

		if info.Size() != int64(len(content)) {
			t.Errorf("expected size %d, got %d", len(content), info.Size())
		}

		// Verify we can read it back
		reader, err := s3FS.Read(ctx, key)
		if err != nil {
			t.Fatalf("failed to read from S3: %v", err)
		}
		defer reader.Close()

		readContent, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("failed to read content: %v", err)
		}

		if len(readContent) != len(content) {
			t.Errorf("size mismatch: expected %d, got %d", len(content), len(readContent))
		}
	})
}

// testWriter is a simple writer that buffers up to max bytes for testing
type testWriter struct {
	buffer *[]byte
	max    int
}

func (w *testWriter) Write(p []byte) (n int, err error) {
	if len(*w.buffer) >= w.max {
		return len(p), nil // Don't buffer more, but pretend we wrote it
	}

	remaining := w.max - len(*w.buffer)
	if remaining > len(p) {
		remaining = len(p)
	}
	*w.buffer = append(*w.buffer, p[:remaining]...)
	return len(p), nil
}
