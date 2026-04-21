.PHONY: build test fmt lint

build:
	go build -o go-quickjs .

test:
	go test ./...

fmt:
	go fmt ./...

lint:
	golangci-lint run ./...
