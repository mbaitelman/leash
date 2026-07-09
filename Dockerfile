# Build on the native host arch to avoid QEMU emulation; Go cross-compiles to TARGETARCH natively.
FROM --platform=$BUILDPLATFORM golang:1.26.4-alpine AS builder
ARG TARGETARCH
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -ldflags="-s -w" -o /leash ./cmd/leash

FROM scratch
COPY --from=builder /leash /leash
# Required for HTTPS calls to the Datadog API and Slack webhooks
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
# Run as a non-root user. Note for `serve` mode: a bind-mounted runs directory
# must be writable by uid 65532 (e.g. mkdir -p runs && chown 65532 runs).
USER 65532:65532
ENTRYPOINT ["/leash"]
CMD ["run"]
