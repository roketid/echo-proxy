FROM golang:1.21 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN go build -o echoproxy cmd/main.go

# Create the final image
FROM debian:bullseye-slim

# Create a non-root user
RUN groupadd -r appuser && useradd -r -g appuser appuser

WORKDIR /app/

COPY --from=builder /app/echoproxy /usr/local/bin/echoproxy
COPY ./config/config.example.json /app/config.json

# Set ownership and permissions
RUN chown -R appuser:appuser /app && chmod -R 755 /app

# Switch to non-root user
USER appuser

ENV PORT=8080
CMD ["/usr/local/bin/echoproxy", "-config", "/app/config.json"]
