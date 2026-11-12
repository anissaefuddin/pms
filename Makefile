.PHONY: gen generate test lint build dev

gen:
	go tool oapi-codegen --config api/oapi-codegen.yaml api/openapi.yaml
	sqlc generate
	cd web && npm run generate

generate: gen

test:
	go test -count=1 ./server/...
	cd web && npm run typecheck && npm test

lint:
	@unformatted=$$(gofmt -l server) && { [ -z "$$unformatted" ] || { printf '%s\n' "$$unformatted"; exit 1; }; }
	go vet ./server/...
	cd web && npm run lint

build:
	cd web && npm run build
	go build ./server/cmd/pms

dev:
	@set -eu; \
	dev_dir=$$(mktemp -d); \
	backend= frontend=; \
	trap 'kill $${backend:-} $${frontend:-} 2>/dev/null || :; wait; rm -rf "$$dev_dir"' EXIT; \
	trap 'exit 130' INT; \
	trap 'exit 143' TERM; \
	go build -o "$$dev_dir/pms" ./server/cmd/pms; \
	PUBLIC_URL="$${PUBLIC_URL:-http://localhost:5173}" "$$dev_dir/pms" serve & backend=$$!; \
	(cd web && exec ./node_modules/.bin/vite --strictPort) & frontend=$$!; \
	while kill -0 "$$backend" 2>/dev/null && kill -0 "$$frontend" 2>/dev/null; do sleep 1; done; \
	if kill -0 "$$backend" 2>/dev/null; then wait "$$frontend"; else wait "$$backend"; fi
