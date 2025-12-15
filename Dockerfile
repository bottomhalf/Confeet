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

# Copy config files
COPY --from=builder /app/shared/config.dev.json ./shared/
COPY --from=builder /app/shared/config.prod.json ./shared/

# Set config path environment variable (use prod config in production)
ENV CONFIG_PATH=./shared/config.prod.json

# After build stage, before CMD
# COPY ./frontend /frontend

# Expose ports (socket server on 8402)
EXPOSE 8402

# Run the server
CMD ["./confeet"]
