.PHONY: setup build run clean test

# ==========================================
# Central vault directories
# ==========================================
BIN_DIR := bin
DATA_DIR := data
LOG_DIR := logs
CACHE_DIR := .go-cache

# ==========================================
# Executable extension
# ==========================================
ifeq ($(OS),Windows_NT)
EXEEXT := .exe
else
EXEEXT :=
endif

APP_BIN := $(BIN_DIR)/app$(EXEEXT)

# ==========================================
# Trap Go downloads inside the project vault
# ==========================================
export GOMODCACHE := $(CURDIR)/$(CACHE_DIR)/mod
export GOCACHE := $(CURDIR)/$(CACHE_DIR)/build
export GO111MODULE := on

# ==========================================
# Central application paths
# ==========================================
export APP_DB_PATH := $(CURDIR)/$(DATA_DIR)/user_auth.db
export APP_LOG_PATH := $(CURDIR)/$(LOG_DIR)/app.log

setup:
	@echo "Creating project vault..."
	@mkdir -p $(BIN_DIR) $(DATA_DIR) $(LOG_DIR) $(CACHE_DIR)/mod $(CACHE_DIR)/build
	@if [ ! -f go.mod ]; then go mod init user-auth-api-dashboard; fi
	@go mod tidy

build: setup
	@echo "Building executable..."
	go build -o $(APP_BIN) ./cmd/main.go

run: build
	@echo "Starting server..."
	./$(APP_BIN)

test:
	go test ./...

clean:
	@echo "Cleaning vault..."
	rm -rf $(BIN_DIR) $(LOG_DIR) $(CACHE_DIR)