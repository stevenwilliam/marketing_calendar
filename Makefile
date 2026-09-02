# marketing_calendar
#
# Configuration lives at /etc/marketing_calendar/marketing_calendar.env, not in
# the repo. Every target that touches the database sources it, so there is one
# place secrets come from and none of them are here.

BIN  := bin/mc
ENV  := /etc/marketing_calendar/marketing_calendar.env
LOAD := set -a; . $(ENV); set +a;

.PHONY: help build web run migrate-up migrate-down migrate-status seed job \
        test test-shuffle vet fmt contrast audit typecheck check deploy clean

help: ## show this
	@grep -E '^[a-z-]+:.*?##' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  %-16s %s\n",$$1,$$2}'

web: ## build the frontend (embedded into the binary, so this runs FIRST)
	cd web && npm ci && npm run build

build: ## build the binary (run `make web` first if the UI changed)
	go build -o $(BIN) ./cmd/api

run: build ## run the HTTP service in the foreground
	$(LOAD) $(BIN) serve

migrate-up: build ## apply pending migrations
	$(LOAD) $(BIN) migrate up

migrate-down: build ## roll back the most recent migration (development only)
	$(LOAD) $(BIN) migrate down

migrate-status: build ## show applied and pending migrations
	$(LOAD) $(BIN) migrate status

seed: build ## load reference and demo data (idempotent)
	$(LOAD) $(BIN) seed

job: build ## run a job, e.g. make job JOB=auto-cancel
	$(LOAD) $(BIN) job $(JOB)

vet: ## go vet
	go vet ./...

fmt: ## gofmt every package, and fail if anything was unformatted
	@out=$$(gofmt -l cmd internal test db web 2>/dev/null); \
	if [ -n "$$out" ]; then echo "unformatted:"; echo "$$out"; exit 1; fi
	@echo "gofmt clean"

test: ## the full suite (integration tests need MC_TEST_DATABASE_URL)
	$(LOAD) go test ./... -count=1

test-shuffle: ## the suite, shuffled — order dependence is a defect
	$(LOAD) go test ./... -count=1 -shuffle=on

typecheck: ## TypeScript, no emit
	cd web && npx tsc --noEmit

audit: ## npm advisories
	cd web && npm audit --audit-level=moderate

contrast: ## measure every colour pairing against design.md
	python3 scripts/contrast.py

check: fmt vet test contrast typecheck audit ## everything that gates a commit

deploy: web build migrate-up ## build both halves, migrate, then restart
	sudo systemctl restart marketing-calendar
	@sleep 2
	@curl -fsS http://127.0.0.1:8093/readyz && echo
	@systemctl is-active marketing-calendar

clean:
	rm -rf bin web/dist/assets web/dist/index.html
