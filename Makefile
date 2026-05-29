.PHONY: all test lint build docker check clean tools fmt vuln actionlint tidy coverage coverage-check precommit fix deadcode \
	help integration integration-up integration-up-auto integration-test integration-down integration-reset \
	integration-fresh integration-fresh-auto integration-auto integration-logs \
	hass-dev hass-up hass-test hass-down hass-reset

PRE_COMMIT := $(shell command -v pre-commit 2>/dev/null)
ifeq ($(PRE_COMMIT),)
PRE_COMMIT := $(shell python3 -m pre_commit --version >/dev/null 2>&1 && echo "python3 -m pre_commit")
endif
GOLANGCI_LINT := $(shell command -v golangci-lint 2>/dev/null)
GOVULNCHECK := $(shell command -v govulncheck 2>/dev/null)

ifeq ($(GOLANGCI_LINT),)
GOLANGCI_LINT := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
endif

ifeq ($(GOVULNCHECK),)
GOVULNCHECK := go run golang.org/x/vuln/cmd/govulncheck@latest
endif

BINARY_NAME=threadgate
SRC_DIR=src/manager
GO_TEST_FLAGS ?= -shuffle=on
COVERAGE_THRESHOLD ?= 0

all: check build

test:
	@echo "🔍 Running tests..."
	cd $(SRC_DIR) && go test $(GO_TEST_FLAGS) -v -coverprofile=../../coverage.out ./...

coverage: test
	@echo "📊 Generating coverage report..."
	go tool cover -html=coverage.out

coverage-check: test
	@echo "📈 Checking coverage threshold ($(COVERAGE_THRESHOLD)%)..."
	@TOTAL_COVERAGE=$$(cd $(SRC_DIR) && go tool cover -func=../../coverage.out | grep total | awk '{print substr($$3, 1, length($$3)-1)}'); \
	echo "Total coverage: $$TOTAL_COVERAGE%"; \
	if [ -z "$$TOTAL_COVERAGE" ]; then \
		echo "ℹ️ No tests executed or coverage recorded."; \
	elif [ $$(echo "$$TOTAL_COVERAGE < $(COVERAGE_THRESHOLD)" | bc) -eq 1 ]; then \
		echo "❌ Coverage is below $(COVERAGE_THRESHOLD)%"; \
		exit 1; \
	fi
	@echo "✅ Coverage check passed!"

lint:
	@echo "✨ Running linter..."
	cd $(SRC_DIR) && $(GOLANGCI_LINT) run --timeout 5m --config ../../.golangci.yml ./...

precommit:
	@echo "🎨 Running all pre-commit hooks..."
	@if [ -n "$(PRE_COMMIT)" ]; then \
		$(PRE_COMMIT) run --all-files; \
	else \
		echo "❌ pre-commit not found; install with: pip install pre-commit && pre-commit install"; \
		exit 1; \
	fi

vuln:
	@echo "🛡️  Checking for vulnerabilities..."
	cd $(SRC_DIR) && $(GOVULNCHECK) ./...

actionlint:
	@echo "🤖 Checking GitHub Actions..."
	@if command -v actionlint &> /dev/null; then \
		actionlint; \
	else \
		echo "actionlint not found, skipping..."; \
	fi

fmt:
	@echo "🧹 Formatting code..."
	cd $(SRC_DIR) && go fmt ./...

tidy:
	@echo "📦 Tidying modules..."
	cd $(SRC_DIR) && go mod tidy

build:
	@echo "🔨 Building binary..."
	cd $(SRC_DIR) && CGO_ENABLED=0 go build -ldflags "-s -w" -o ../../$(BINARY_NAME) main.go

docker-build:
	@echo "🐳 Building Docker image (local)..."
	docker compose build

# --- Home Assistant + ThreadGate local integration (docker-compose.integration.yml) ---
#
#   make integration             # manual pairing (dashboard approve) + smoke tests
#   make integration-auto        # auto-approve pairing + smoke tests (CI / other features)
#   make integration-fresh       # wipe state, manual pairing, tests
#   make integration-fresh-auto  # wipe state, auto pairing, tests
#   make integration-down        # stop containers (keep testdata for fast next up)

help:
	@echo "ThreadGate Make targets"
	@echo ""
	@echo "  make integration              Manual pairing (default) + smoke tests"
	@echo "  make integration-auto         Auto-approve pairing + smoke tests"
	@echo "  make integration-fresh        Reset testdata, manual pairing, tests"
	@echo "  make integration-fresh-auto   Reset testdata, auto pairing, tests"
	@echo "  make integration-up           Start/configure only (manual pairing)"
	@echo "  make integration-up-auto      Start/configure only (auto pairing)"
	@echo "  make integration-test         Run smoke tests (stack must already be up)"
	@echo "  make integration-down    Stop integration containers"
	@echo "  make integration-reset   Stop and remove local HA/ThreadGate test state"
	@echo "  make integration-logs    Follow container logs (LOGS=ha|tg|all)"
	@echo ""
	@echo "  make all                 Run checks and build binary"
	@echo "  make test                Run Go unit tests"

integration: integration-up integration-test
	@echo ""
	@echo "✅ Home Assistant + ThreadGate integration is ready"
	@echo "   Home Assistant:  http://127.0.0.1:8123"
	@echo "   ThreadGate:      http://127.0.0.1:8081"
	@echo "   Credentials:     testdata/ha-credentials.env"

integration-up:
	@./scripts/hass-dev.sh up

integration-up-auto:
	@./scripts/hass-dev.sh up-auto

integration-test:
	@./scripts/hass-dev.sh test

integration-auto: integration-up-auto integration-test
	@echo ""
	@echo "✅ Integration ready (pairing auto-approved)"
	@echo "   Home Assistant:  http://127.0.0.1:8123"
	@echo "   ThreadGate:      http://127.0.0.1:8081"

integration-fresh:
	@./scripts/hass-dev.sh cycle

integration-fresh-auto:
	@./scripts/hass-dev.sh cycle-auto

integration-down:
	@./scripts/hass-dev.sh down

integration-reset:
	@./scripts/hass-dev.sh reset

integration-logs:
	@./scripts/hass-dev.sh logs $(or $(LOGS),all)

# Aliases (same scripts)
hass-dev: integration-fresh-auto
hass-up: integration-up
hass-up-auto: integration-up-auto
hass-test: integration-test
hass-down: integration-down
hass-reset: integration-reset

fix: tidy fmt
	@echo "🔧 Running go fix..."
	cd $(SRC_DIR) && go fix ./...
	@echo "🔧 Auto-fixing linter issues..."
	-cd $(SRC_DIR) && $(GOLANGCI_LINT) run --fix --timeout 5m --config ../../.golangci.yml ./...

deadcode:
	@echo "💀 Checking for dead code..."
	cd $(SRC_DIR) && go run golang.org/x/tools/cmd/deadcode@latest ./...

check: tidy fmt lint vuln actionlint coverage-check deadcode
	@echo "✅ All local checks passed!"

tools:
	@echo "🛠️  Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest
	go install golang.org/x/tools/cmd/deadcode@latest
	@if ! command -v actionlint &> /dev/null; then \
		if [[ "$$OSTYPE" == "darwin"* ]]; then \
			brew install actionlint; \
		else \
			go install github.com/rhysd/actionlint/cmd/actionlint@latest; \
		fi \
	fi
	@if [ -z "$(PRE_COMMIT)" ]; then \
		pip install pre-commit; \
	fi

clean:
	@echo "🧹 Cleaning compiled artifacts..."
	rm -f $(BINARY_NAME)
	rm -f $(SRC_DIR)/manager
	rm -f coverage.out
	rm -f $(SRC_DIR)/coverage.out
	rm -f coverage.html
