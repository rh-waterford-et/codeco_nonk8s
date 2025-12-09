# Build stage
FROM golang:1.25.4-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /workspace

# Copy go mod files and vendor directory
COPY go.mod go.sum ./
COPY vendor/ vendor/

# Copy source code
COPY . .

# Build the binary using vendored dependencies (no network required)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -mod=vendor \
    -ldflags="-w -s" \
    -o vk-flightctl-provider \
    ./cmd/vk-flightctl-provider

# Runtime stage
FROM gcr.io/distroless/static:nonroot

# Copy CA certificates from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the binary with proper permissions for OpenShift
# OpenShift runs with random UID but group 0 (root group)
COPY --from=builder --chown=65532:0 --chmod=0750 \
    /workspace/vk-flightctl-provider /usr/local/bin/vk-flightctl-provider

# OpenShift will override this with a random UID from the namespace range
# We still set it for compatibility with standard Kubernetes
USER 65532:0

# Set entrypoint
ENTRYPOINT ["/usr/local/bin/vk-flightctl-provider"]
