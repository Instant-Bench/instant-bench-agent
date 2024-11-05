# Compiler
GO := go

# Directory containing the Go source code
SRC_DIR := cli

# Name of the binary to be built
BINARY_NAME := ib-agent-cli

# Global path to move the binary
GLOBAL_BIN_PATH := /usr/local/bin

.PHONY: build install clean

build:
	@echo "Building $(BINARY_NAME)..."
	TERRAFORM_PATH=$(shell pwd)/aws && \
	cd $(SRC_DIR) && \
	$(GO) mod download && \
	$(GO) build -ldflags "-X main.terraformPath=$$TERRAFORM_PATH" -o $(BINARY_NAME)

install: build
	@echo "Installing $(BINARY_NAME) to $(GLOBAL_BIN_PATH)..."
	@sudo mv $(SRC_DIR)/$(BINARY_NAME) $(GLOBAL_BIN_PATH)/$(BINARY_NAME)

clean:
	@echo "Cleaning up..."
	@rm -f $(GLOBAL_BIN_PATH)/$(BINARY_NAME)
