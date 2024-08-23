.PHONY: test
test:
	go test ./... -v

.PHONY: lint
lint:
	golangci-lint run

.PHONY: tidy
tidy:
	go mod tidy