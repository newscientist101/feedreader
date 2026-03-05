.PHONY: build clean stop start restart test lint lint-go lint-js lint-css lint-templates lint-html fmt fmt-check fix-check vulncheck check layout-test browser-unit-test

build:
	go build -o feedreader ./cmd/srv

clean:
	rm -f feedreader

test:
	go test ./...
	@go test -v -run TestPerformance ./srv/ 2>&1 | grep -E '(median=|FAIL|PASS)'
	NO_COLOR=1 npx vitest run --config tests/config/vitest.config.mjs

lint: lint-go lint-js lint-css lint-templates lint-html

lint-go:
	golangci-lint run ./...

lint-js:
	npx eslint --config tests/config/eslint.config.mjs srv/static/

lint-css:
	npx stylelint srv/static/**/*.css

lint-templates:
	go run ./cmd/lint-templates/ srv/templates/

lint-html:
	djlint srv/templates/ --lint

fmt:
	goimports -w .

fmt-check:
	@test -z "$$(goimports -l .)" || (echo "goimports needed on:"; goimports -l .; exit 1)

fix-check:
	@test -z "$$(go fix -diff ./... 2>&1)" || (echo "go fix has suggestions:"; go fix -diff ./...; exit 1)

vulncheck:
	govulncheck ./...

layout-test:
	NO_COLOR=1 npx vitest run --config tests/config/vitest.browser.config.mjs

browser-unit-test:
	NO_COLOR=1 npx vitest run --config tests/config/vitest.browser-unit.config.mjs

check: fmt-check fix-check lint test vulncheck
