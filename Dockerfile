# Build stage
FROM golang:1.24-alpine AS builder

ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary with version info
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
    -o elava ./cmd/elava

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

# Copy binary from builder to /usr/local/bin
COPY --from=builder /build/elava /usr/local/bin/elava

# Expose metrics port
EXPOSE 9090

# Run the daemon by default
ENTRYPOINT ["/usr/local/bin/elava"]
