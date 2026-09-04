.PHONY: build test web

web:
	cd web && pnpm build

build: web
	go build ./cmd/roosterd
	go build ./cmd/rooster

test: web
	go test ./...
	cd web && pnpm lint
