Http server serving static files
====

This project aims at providing a basic http server in a docker container for serving static files.
It is available for amd64, arm64 and arm/v7 platforms.

The latest image is published to GitHub Container Registry on every push to `main`:

```sh
docker pull ghcr.io/touilleio/http-server-static-files:latest
```

# Usage

```sh
docker run --rm -v "${PWD}/static:/static:ro" -p 8080:8080 ghcr.io/touilleio/http-server-static-files:latest
```

## Configuration

The configuration can be set via environment variables, defined in the table below.

| Variable name | Default value | Details |
|---------------|---------------|-------------|
| `PORT`        | `8080`        | Port the http server listens on. |
| `ROOT_PATH`   | `/static`     | Path inside the container the http server is serving. Note that the process runs as `nobody`, folders and files in this path must be readable by `nobody`|

An example of changing the configuration via environment variable is provided below:

```sh
docker run --rm -v "${PWD}/web:/web:ro" -e ROOT_PATH=/web -e PORT=8081 -p 8081:8081 ghcr.io/touilleio/http-server-static-files:latest
```
