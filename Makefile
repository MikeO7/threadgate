.PHONY: all test lint build docker check clean tools fmt vuln actionlint tidy coverage coverage-check precommit fix deadcode

PRE_COMMIT := $(shell command -v pre-commit 2>/dev/null)
ifeq ($(PRE_COMMIT),)
PRE_COMMIT := $(shell python3 -m pre_commit --version >/dev/null 2>&1 && echo "python3 -m pre_commit")
endif
GOLANGCI_LINT := $(shell command -v golangci-lint 2>/dev/null)
GOVULNCHECK := $(shell command -v govulncheck 2>/dev/null)

ifeq ($(GOLANGCI_LINT),)
GOLANGCI_LINT := go run github.com/golangci/golangci-lint/cmd/golangci-lint@latest
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

fix: tidy fmt
	@echo "🔧 Running go fix..."
	cd $(SRC_DIR) && go fix ./...
	@echo "🔧 Auto-fixing linter issues..."
	-cd $(SRC_DIR) && $(GOLANGCI_LINT) run --fix --timeout 5m --config ../../.golangci.yml ./...

deadcode:
	@echo "💀 Checking for dead code..."
	cd $(SRC_DIR) && go run golang.org/x/tools/cmd/deadcode@latest ./...

check: tidy fmt lint vuln actionlint coverage-check
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
