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
ENTRYPOINT ["/leash"]
CMD ["run"]
