FROM golang:1.26.4-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /leash ./cmd/leash

FROM scratch
COPY --from=builder /leash /leash
# Required for HTTPS calls to the Datadog API and Slack webhooks
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT ["/leash"]
CMD ["run"]
