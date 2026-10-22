# sms-platform — course demo project (COURSE-PLAN § 7.1).
# `make test` is the acceptance gate for every work order (AGENTS.md).
GO ?= go

.PHONY: build vet test e2e run

build:
	$(GO) build -o bin/ ./...

vet:
	$(GO) vet ./...

test:
	$(GO) test ./... -race

e2e:
	$(GO) test ./e2e/... -race -v -run 'TestE2E_'

run:
	$(GO) run ./cmd/smsd
