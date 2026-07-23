.PHONY: build check demo fmt fmt-check lint mod-verify test test-race tidy-check vet

build:
	mkdir -p dist
	go build -trimpath -o ./dist/tren ./cmd/tren

fmt:
	gofmt -w .

fmt-check:
	@files="$$(gofmt -l .)"; \
	if [ -n "$$files" ]; then \
		echo "These Go files need formatting:"; \
		echo "$$files"; \
		exit 1; \
	fi

tidy-check:
	go mod tidy -diff

mod-verify:
	go mod verify

vet:
	go vet ./...

lint: fmt-check tidy-check mod-verify vet

test:
	go test ./...

test-race:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...

demo:
	go run ./cmd/tren stats bench --demo >/dev/null

check: lint test-race build demo
