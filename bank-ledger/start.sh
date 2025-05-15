#!/bin/bash

set -e

echo "Checking for Docker..."
if ! command -v docker &> /dev/null; then
    echo "Docker is not installed. Please install Docker first."
    echo "Visit https://docs.docker.com/get-docker/ for installation instructions."
    exit 1
fi

echo "Checking for Docker Compose..."
# First check for docker compose (v2)
if docker compose version &> /dev/null; then
    echo "Docker Compose v2 found."
    COMPOSE_COMMAND="docker compose"
# Then check for docker-compose (v1)
elif command -v docker-compose &> /dev/null; then
    echo "Docker Compose v1 found."
    COMPOSE_COMMAND="docker-compose"
else
    echo "Docker Compose not found. Installing Docker Compose v2..."

    # This is a simplified install - for most systems, Docker Compose is included with Docker Desktop
    # or can be installed through package managers

    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    if [ "$OS" = "darwin" ]; then
        echo "On macOS, Docker Compose should be installed with Docker Desktop."
        echo "Please install Docker Desktop from https://docs.docker.com/desktop/mac/install/"
        exit 1
    elif [ "$OS" = "linux" ]; then
        echo "Installing Docker Compose plugin for Linux..."
        DOCKER_CONFIG=${DOCKER_CONFIG:-$HOME/.docker}
        mkdir -p $DOCKER_CONFIG/cli-plugins

        if [ "$ARCH" = "x86_64" ]; then
            COMPOSE_URL="https://github.com/docker/compose/releases/latest/download/docker-compose-linux-x86_64"
        elif [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then
            COMPOSE_URL="https://github.com/docker/compose/releases/latest/download/docker-compose-linux-aarch64"
        else
            echo "Unsupported architecture: $ARCH"
            exit 1
        fi

        curl -SL $COMPOSE_URL -o $DOCKER_CONFIG/cli-plugins/docker-compose
        chmod +x $DOCKER_CONFIG/cli-plugins/docker-compose

        # Create a symlink for backward compatibility
        sudo ln -sf $DOCKER_CONFIG/cli-plugins/docker-compose /usr/local/bin/docker-compose

        if docker compose version &> /dev/null; then
            echo "Docker Compose v2 installed successfully."
            COMPOSE_COMMAND="docker compose"
        else
            echo "Installation failed. Please install Docker Compose manually."
            exit 1
        fi
    elif [ "$OS" = "windows" ] || [[ "$OS" == *"mingw"* ]]; then
        echo "On Windows, Docker Compose should be installed with Docker Desktop."
        echo "Please install Docker Desktop from https://docs.docker.com/desktop/windows/install/"
        exit 1
    else
        echo "Unsupported OS: $OS"
        exit 1
    fi
fi

echo "Detecting platform..."
ARCH=$(uname -m)
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
echo "Detected OS=$OS, ARCH=$ARCH"

# Set GOARCH and GOOS based on detected platform
if [[ "$ARCH" == "x86_64" ]]; then
  GOARCH=amd64
elif [[ "$ARCH" == "arm64" || "$ARCH" == "aarch64" ]]; then
  GOARCH=arm64
else
  echo "Unsupported architecture: $ARCH"
  exit 1
fi

if [[ "$OS" == "darwin" ]]; then
  GOOS=darwin
elif [[ "$OS" == "linux" ]]; then
  GOOS=linux
else
  echo "Unsupported OS: $OS"
  exit 1
fi

echo "Building Go binaries for $GOOS/$GOARCH..."

# Build bank-ledger-service
GOARCH=$GOARCH GOOS=$GOOS go build -o bank-ledger-service ./cmd/bank-ledger-service
echo "Built bank-ledger-service binary."

# Build bank-ledger-consumer
GOARCH=$GOARCH GOOS=$GOOS go build -o bank-ledger-consumer ./cmd/bank-ledger-consumer
echo "Built bank-ledger-consumer binary."

echo "Starting Docker dependencies..."
$COMPOSE_COMMAND down
$COMPOSE_COMMAND up -d

echo "Waiting for dependencies to be healthy..."
sleep 20  # Increased wait time to ensure Kafka is fully initialized

echo "Checking if Kafka is up..."
if docker ps | grep -q kafka; then
  echo "Kafka is running."
else
  echo "Kafka container is not running! Check docker logs."
  exit 1
fi

# Check if port 9092 is available
if command -v nc &> /dev/null; then
  nc -z localhost 9092
  if [ $? -eq 0 ]; then
    echo "Kafka port 9092 is open and accepting connections."
  else
    echo "Kafka port 9092 is not accessible! Check Kafka configuration."
    exit 1
  fi
else
  echo "Note: 'nc' command not found, skipping port check."
fi

echo "Running Go services locally..."

./bank-ledger-service &
SERVICE_PID=$!
./bank-ledger-consumer &
CONSUMER_PID=$!

echo "Go services running. PIDs: service=$SERVICE_PID, consumer=$CONSUMER_PID"

wait $SERVICE_PID $CONSUMER_PID
