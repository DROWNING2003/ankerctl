BINARY := bin/ankerctl

.PHONY: all fmt vet test build clean
all: fmt vet test build
fmt:
	gofmt -w .
vet:
	go vet ./...
test:
	go test ./...
build:
	mkdir -p bin
	go build -o $(BINARY) ./cmd/ankerctl
clean:
	rm -rf bin
