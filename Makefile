.PHONY: build clean stop start restart test lint lint-go lint-js lint-templates lint-html fmt fmt-check vulncheck check

build:
	go build -o feedreader ./cmd/srv

clean:
	rm -f feedreader

test:
	go test ./...

lint: lint-go lint-js lint-templates lint-html

lint-go:
	golangci-lint run ./...

lint-js:
	eslint srv/static/

lint-templates:
	go run ./cmd/lint-templates/ srv/templates/

lint-html:
	djlint srv/templates/ --lint

fmt:
	goimports -w .

fmt-check:
	@test -z "$$(goimports -l .)" || (echo "goimports needed on:"; goimports -l .; exit 1)

vulncheck:
	govulncheck ./...

check: fmt-check lint test vulncheck
