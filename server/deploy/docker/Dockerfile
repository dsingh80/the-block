# Multi-stage build producing both server binaries from one image
# (guidelines/06-backend-architecture.md, "Deployment & HTTPS") -- cmd/api and
# cmd/reconcile share the same module, so one build stage covers both; which
# binary a given container actually runs is just a docker-compose `command:`
# override, not a separate image.

FROM golang:1.25-alpine AS build
WORKDIR /src

# Dependencies in their own layer so `go mod download` only reruns when
# go.mod/go.sum actually change, not on every source edit.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api
RUN CGO_ENABLED=0 go build -o /out/reconcile ./cmd/reconcile

# A minimal runtime image -- no Go toolchain, no shell utilities beyond what
# TLS verification needs (ca-certificates, for the app's own outbound
# connections; Caddy is what terminates inbound TLS, guidelines/06-backend-architecture.md).
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /out/api /usr/local/bin/api
COPY --from=build /out/reconcile /usr/local/bin/reconcile

EXPOSE 8080
# No ENTRYPOINT: each docker-compose service supplies the full command
# ("api" or "reconcile") itself. An ENTRYPOINT here would make compose's
# per-service `command:` append an argument to it rather than choosing which
# binary to run at all -- exactly the gotcha that made a first pass at this
# file run the long-lived api server under the reconcile service, which then
# never exited (guidelines/06-backend-architecture.md).
CMD ["api"]
