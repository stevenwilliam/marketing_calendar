# marketing_calendar
BIN := bin/mc
ENV := $(CURDIR)/.env

.PHONY: help build run migrate status seed job test test-shuffle vet contrast check clean

help:
	@grep -E '^[a-z-]+:.*?##' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  %-14s %s\n",$$1,$$2}'

build: ## build the binary
	go build -o $(BIN) ./cmd/api

run: build ## run the HTTP server
	$(BIN) serve

migrate: build ## apply pending migrations
	$(BIN) migrate

status: build ## show migration status
	$(BIN) migrate:status

seed: build ## load demo data
	$(BIN) seed

job: build ## run a scheduled job, e.g. make job JOB=auto-cancel-promos
	$(BIN) job $(JOB)

vet: ## go vet
	go vet ./...

test: ## the full suite
	go test ./...

test-shuffle: ## integration suite, shuffled — order dependence is a defect
	set -a; . $(ENV); set +a; go test ./test/... -count=1 -shuffle=on

contrast: ## measure every colour pairing against the sheet
	python3 scripts/contrast.py

check: vet test contrast ## everything that gates a commit

clean:
	rm -rf bin dist
