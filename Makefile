all: clean test vet check-gofmt lint

check-gofmt:
	scripts/check_gofmt.sh

clean:
	go clean ./...

lint:
	$(shell go env GOPATH)/bin/golint -set_exit_status ./...

test:
	go test ./...

test-nocache:
	go test -count=1 ./...

vet:
	go vet ./...
