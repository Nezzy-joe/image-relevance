.PHONY: run fmt test vet check compose-up compose-down

run:
	go run ./cmd/api

fmt:
	gofmt -w $$(find cmd internal -name '*.go' -type f)

test:
	go test ./...

vet:
	go vet ./...

check: fmt test vet

compose-up:
	docker compose up --build

compose-down:
	docker compose down
