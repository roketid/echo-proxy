# Use the official Golang image as the builder stage
FROM golang:alpine as builder

# Set the working directory
WORKDIR /app

# Copy go modules files
COPY go.mod go.sum ./

# Download the Go modules
RUN go mod download

# Copy the entire source code
COPY . .

# Build the Go app
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd/main.go

# Start a new stage from scratch
FROM alpine:latest

# Install necessary dependencies (if any)
RUN apk add --no-cache ca-certificates

# Create a non-root user
RUN addgroup -S appuser && adduser -S appuser -G appuser

WORKDIR /app/

# Copy the built binary from the builder stage
COPY --from=builder /app/main .
COPY ./config/config.example.json ./config.json

# Set ownership and permissions
RUN chown -R appuser:appuser /app && chmod -R 755 /app

# Switch to non-root user
USER appuser

ENV PORT=8080

# Command to run
CMD ["./main", "-config", "config.json"]
