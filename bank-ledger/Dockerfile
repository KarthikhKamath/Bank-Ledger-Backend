# Step 1: Build the Go application
FROM golang:1.22 AS builder

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Show the Go version and the go.mod file for debugging
RUN go version && echo "Original go.mod:" && cat go.mod

# Modify go.mod to use Go 1.22 instead of 1.23
RUN sed -i -E 's/^go 1\.23.*/go 1.22/' go.mod && \
    echo "Modified go.mod:" && cat go.mod && \
    # Also update any dependencies if needed
    sed -i 's/gorm.io\/gorm v1.26.1/gorm.io\/gorm v1.25.7/' go.mod

# Download all dependencies
RUN go mod download

# Copy the entire project
COPY . .

# List files to ensure everything is copied correctly
RUN ls -la

# Build the Go application for bank-ledger-service
RUN cd cmd/bank-ledger-service && CGO_ENABLED=0 go build -o /app/bank-ledger-service main.go || { echo "Failed to build bank-ledger-service"; ls -la; exit 1; }

# Build the Go application for bank-ledger-consumer
RUN cd cmd/bank-ledger-consumer && CGO_ENABLED=0 go build -o /app/bank-ledger-consumer main.go || { echo "Failed to build bank-ledger-consumer"; ls -la; exit 1; }

# Step 2: Create the final image with the binaries
FROM debian:bullseye-slim

# Install required dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    tzdata \
    && rm -rf /var/lib/apt/lists/*

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy the pre-built binaries from the builder stage
COPY --from=builder /app/bank-ledger-service /app/bank-ledger-service
COPY --from=builder /app/bank-ledger-consumer /app/bank-ledger-consumer

# Make the binaries executable
RUN chmod +x /app/bank-ledger-service /app/bank-ledger-consumer

# Expose the ports used by your app
EXPOSE 8001 9000 8002

# Environment variables (can be customized)
ENV MYSQL_HOST=db
ENV MONGO_HOST=mongodb
ENV REDIS_HOST=redis
ENV KAFKA_BROKER=kafka:9092

# Command to run the Bank Ledger Service
CMD ["sh", "-c", "if [ \"$MODE\" = 'consumer' ]; then /app/bank-ledger-consumer; else /app/bank-ledger-service; fi"]