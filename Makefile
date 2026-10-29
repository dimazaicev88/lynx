.PHONY: build test fmt vet clean

build:
	go build -o bin/lynxc ./cmd/lynxc

test:
	go test ./...

fmt:
	gofmt -w cmd internal

vet:
	go vet ./...

clean:
	rm -rf bin
