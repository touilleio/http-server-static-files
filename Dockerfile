# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.27.3-trixie@sha256:fbb7ae40b62ff8afe31662df570d39b4f5052ac32b90dae8e60d7657a20a74f4 AS builder

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY main.go ./

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build \
      -trimpath \
      -ldflags="-s -w -X main.GitCommit=$GIT_COMMIT -X main.BuildDate=$BUILD_DATE -X main.Version=$VERSION" \
      -o /http-server-static-files \
      .

FROM gcr.io/distroless/static-debian13:nonroot@sha256:d62067879de890a1def9da0e7766b7d20810a5e5555cef8506b752437d7a3e88

COPY --from=builder --chown=nonroot:nonroot /http-server-static-files /http-server-static-files
COPY --chown=nonroot:nonroot static/index.html /static/index.html

USER nonroot:nonroot

ENTRYPOINT ["/http-server-static-files"]
EXPOSE 8080
