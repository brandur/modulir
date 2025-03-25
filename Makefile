all: clean test vet check-gofmt lint

.PHONY: check-gofmt
check-gofmt:
	scripts/check_gofmt.sh

.PHONY: clean
clean:
	go clean ./...

.PHONY: lint
lint:
	golangci-lint run --fix

.PHONY: test
test:
	go test ./... -test.v

.PHONY: test-nocache
test-nocache:
	go test -count=1 ./...

.PHONY: vet
vet:
	go vet ./...
