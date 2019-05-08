all: test vet check-gofmt lint

check-gofmt:
	scripts/check_gofmt.sh

lint:
	$(GOPATH)/bin/golint -set_exit_status `go list ./... | grep -v /vendor/`

test:
	go test ./...

vet:
	go vet ./...
