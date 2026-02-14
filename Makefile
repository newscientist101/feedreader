.PHONY: build clean stop start restart test lint lint-go lint-js

build:
	go build -o feedreader ./cmd/srv

clean:
	rm -f feedreader

test:
	go test ./...

lint: lint-go lint-js

lint-go:
	golangci-lint run ./...

lint-js:
	eslint srv/static/
