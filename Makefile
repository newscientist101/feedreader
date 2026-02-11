.PHONY: build clean stop start restart test

build:
	go build -o feedreader ./cmd/srv

clean:
	rm -f feedreader

test:
	go test ./...
