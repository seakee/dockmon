# Project name
PROJECT_NAME ?= dockmon

# Docker image name
IMAGE_NAME ?= $(PROJECT_NAME):latest

# Configuration directory
CONFIG_DIR ?= $(shell pwd)/bin/configs

# Go build flags
GO_FLAGS = -ldflags="-s -w"

# Targets
.PHONY: all test build run docker-build docker-run clean

# Default target that includes formatting, linting, testing, and building
all: fmt build

# Format the source code
fmt:
	@echo "Running gofmt..."
	@gofmt -w .  # Format all Go files in the current directory
	@echo "Running goimports..."
	@goimports -w .  # Run goimports to organize imports

# Build the executable
build: fmt
	@echo "Building binary..."
	@mkdir -p ./bin  # Ensure the bin directory exists
	@go build $(GO_FLAGS) -o ./bin/$(PROJECT_NAME) ./main.go  # Build the Go binary

# Run the application
run:
	@echo "Running application..."
	@./bin/$(PROJECT_NAME)  # Run the compiled binary

# Build the Docker image
docker-build:
	@echo "Building Docker image..."
	@docker build -t $(IMAGE_NAME) .  # Build Docker image with tag go-api:latest

# Run the Docker container
docker-run: docker-clean
	@echo "Running Docker container..."
	@docker run -d --name $(PROJECT_NAME) \
		-p 8085:8080 \
		-it \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v $(CONFIG_DIR):/bin/configs \
		-v /bin/docker:/bin/docker \
		-e APP_NAME=$(PROJECT_NAME) \
		$(IMAGE_NAME)

# Stop and remove existing Docker container with the same name
docker-clean:
	@echo "Stopping and removing existing Docker container..."
	@docker stop $(PROJECT_NAME) 2>/dev/null || true
	@docker rm -f $(PROJECT_NAME) 2>/dev/null || true

# Clean up build artifacts
clean:
	@echo "Cleaning up..."
	@rm -rf ./bin/$(PROJECT_NAME)  # Remove the bin directory