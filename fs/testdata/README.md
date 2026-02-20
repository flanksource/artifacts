# E2E Integration Tests

This directory contains end-to-end integration tests for the artifacts filesystem implementations.

## Prerequisites

- Docker and Docker Compose
- Go 1.24+

## Running Tests

### Start Test Services

First, start the required services using Docker Compose:

```bash
cd fs/testdata
docker compose up -d
```

This will start:
- **LocalStack**: AWS service emulator (S3 on port 4566)
- **MinIO**: S3-compatible storage (ports 9000, 9001)
- **Fake GCS Server**: Google Cloud Storage emulator (port 4443)
- **SFTP Server**: SSH/SFTP server (port 2222)
- **Samba Server**: SMB/CIFS server (port 445)

### Wait for Services to be Ready

```bash
# Check service health
docker compose ps

# Wait for LocalStack to be ready
until curl -f http://localhost:4566/_localstack/health; do sleep 1; done

# Wait for MinIO to be ready
until curl -f http://localhost:9000/minio/health/live; do sleep 1; done
```

### Run All Tests

```bash
# From the repository root
make test

# Or run Go tests directly
go test ./fs -v
```

### Run Only E2E Tests

```bash
# Run S3 E2E tests specifically
go test ./fs -v -run TestS3E2E

# Skip E2E tests (for quick unit tests)
go test ./fs -v -short
```

### Stop Test Services

```bash
cd fs/testdata
docker compose down
```

## Test Structure

### Unit Tests
- `fs_test.go`: General filesystem tests (reads, writes, globs)

### E2E Tests
- `s3_e2e_test.go`: S3-specific end-to-end tests
  - Tests Content-Length header fix
  - Tests with TeeReader chains (simulates artifact saving)
  - Tests with LocalStack and MinIO

## Environment Variables

The following environment variables can be set to customize test behavior:

- `TEST_AWS_ACCESS_KEY_ID`: AWS access key (default: "test" for LocalStack, "minioadmin" for MinIO)
- `TEST_AWS_SECRET_ACCESS_KEY`: AWS secret key (default: "test" for LocalStack, "minioadmin" for MinIO)
- `TEST_AWS_REGION`: AWS region (default: "us-east-1")

## LocalStack vs MinIO

The E2E tests run against both LocalStack and MinIO:

- **LocalStack**: More accurate AWS S3 emulation, better for testing AWS-specific behaviors
- **MinIO**: Lightweight S3-compatible storage, faster startup, good for general S3 API testing

## Troubleshooting

### Tests Fail with Connection Refused
Make sure Docker services are running:
```bash
cd fs/testdata
docker compose ps
docker compose logs
```

### LocalStack Health Check Fails
Check LocalStack logs:
```bash
docker compose logs localstack
```

### Port Conflicts
If ports are already in use, you can modify the port mappings in `docker-compose.yml`.

## CI/CD Integration

The GitHub Actions workflow automatically:
1. Starts Docker Compose services
2. Waits for services to be healthy
3. Runs all tests including E2E tests
4. Stops services and cleans up

See `.github/workflows/test.yml` for the CI configuration.
