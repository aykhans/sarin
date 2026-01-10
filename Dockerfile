ARG GO_VERSION=required

FROM docker.io/library/golang:${GO_VERSION}-alpine AS builder

ARG VERSION=unknown
ARG GIT_COMMIT=unknown

WORKDIR /src

RUN --mount=type=bind,source=./go.mod,target=./go.mod \
    --mount=type=bind,source=./go.sum,target=./go.sum \
    go mod download

RUN --mount=type=bind,source=./,target=./ \
    CGO_ENABLED=0 GOEXPERIMENT=greenteagc go build \
    -ldflags "-X 'go.aykhans.me/sarin/internal/version.Version=${VERSION}' \
    -X 'go.aykhans.me/sarin/internal/version.GitCommit=${GIT_COMMIT}' \
    -X 'go.aykhans.me/sarin/internal/version.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)' \
    -X 'go.aykhans.me/sarin/internal/version.GoVersion=$(go version)' \
    -s -w" \
    -o /sarin ./cmd/cli/main.go

FROM gcr.io/distroless/static-debian12:latest

WORKDIR /

COPY --from=builder /sarin /sarin

ENTRYPOINT ["./sarin"]
