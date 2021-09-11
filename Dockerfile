FROM --platform=$BUILDPLATFORM golang as builder

ARG TARGETOS
ARG TARGETARCH
ARG VERSION

COPY . /src

WORKDIR /src

RUN env GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 GOOS=linux go mod download && \
  export GIT_COMMIT=$(git rev-parse HEAD) && \
  export GIT_DIRTY=$(test -n "`git status --porcelain`" && echo "+CHANGES" || true) && \
  export BUILD_DATE=$(date '+%Y-%m-%d-%H:%M:%S') && \
  env GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 GOOS=linux \
    go build \
    -ldflags "-X main.GitCommit=${GIT_COMMIT}${GIT_DIRTY} \
              -X main.BuildDate=${BUILD_DATE} \
              -X main.Version=${VERSION}" \
              -o http-server-static-files \
    .

FROM --platform=$BUILDPLATFORM gcr.io/distroless/base

COPY --from=builder /src/http-server-static-files /http-server-static-files
COPY static/index.html /static/index.html
USER nobody

ENTRYPOINT ["/http-server-static-files"]
EXPOSE 8080
