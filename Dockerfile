# ------------------------------
# Stage 1: Build
# ------------------------------
FROM golang:1.25 AS builder

# Set the working directory inside the container
WORKDIR /app

# Cache go mod dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the Go binary statically
RUN CGO_ENABLED=0 GOOS=linux go build -o confeet ./cmd/server

# ------------------------------
# Stage 2: Run
# ------------------------------
FROM alpine:3.19

# Set working directory
WORKDIR /root/

# Copy binary from builder
COPY --from=builder /app/confeet .

# After build stage, before CMD
# COPY ./frontend /frontend

# Expose port (replace with your WebSocket port, e.g., 8080)
EXPOSE 8080

# Run the server
CMD ["./confeet"]
