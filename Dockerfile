# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.26.5-trixie AS builder

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

FROM gcr.io/distroless/static-debian13:nonroot

COPY --from=builder --chown=nonroot:nonroot /http-server-static-files /http-server-static-files
COPY --chown=nonroot:nonroot static/index.html /static/index.html

ENTRYPOINT ["/http-server-static-files"]
EXPOSE 8080
