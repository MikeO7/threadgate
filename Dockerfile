# ========================================================
# Stage 1: Build the static Go Orchestrator (threadgate)
# ========================================================
FROM golang:1.25.10-alpine AS builder

WORKDIR /app

# Copy dependency specifications
COPY src/manager/go.mod ./

# Download dependencies
RUN go mod download

# Copy source code
COPY src/manager/ ./

# Statically compile the Go manager to eliminate all external OS library dependencies
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o threadgate main.go

# ========================================================
# Stage 2: Hardened Runtime Environment
# ========================================================
# We layer on top of the official, Google-maintained and certified OTBR core image
FROM openthread/otbr:latest

# Copy our statically compiled Go orchestrator
COPY --from=builder /app/threadgate /usr/local/bin/threadgate

# Secure configurations
RUN chmod +x /usr/local/bin/threadgate

# Define state persistence directory
VOLUME ["/data"]

# Override standard entrypoint with our secure, self-healing Go orchestrator
ENTRYPOINT ["/usr/local/bin/threadgate"]
