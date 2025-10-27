# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o elava ./cmd/elava

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

# Copy binary from builder to /usr/local/bin
COPY --from=builder /build/elava /usr/local/bin/elava

# Expose metrics port
EXPOSE 2112

# Run the daemon by default
ENTRYPOINT ["/usr/local/bin/elava"]
CMD ["daemon"]
