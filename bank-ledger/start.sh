#!/bin/bash

set -e

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
docker-compose down
docker-compose up -d

echo "Waiting for dependencies to be healthy..."
sleep 10

echo "Checking if Kafka is up..."
if docker ps | grep -q kafka; then
  echo "Kafka is running."
else
  echo "Kafka container is not running! Check docker logs."
  exit 1
fi

# Check if port 9092 is available
nc -z localhost 9092
if [ $? -eq 0 ]; then
  echo "Kafka port 9092 is open and accepting connections."
else
  echo "Kafka port 9092 is not accessible! Check Kafka configuration."
  exit 1
fi

echo "Running Go services locally..."

./bank-ledger-service &
SERVICE_PID=$!
./bank-ledger-consumer &
CONSUMER_PID=$!

echo "Go services running. PIDs: service=$SERVICE_PID, consumer=$CONSUMER_PID"

wait $SERVICE_PID $CONSUMER_PID