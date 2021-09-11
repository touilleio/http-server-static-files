Http server serving static files
====

This project aims at providing a basic http server in a docker container for serving static files.
It is available for amd64, arm64 and arm/v7 platforms.

# Usage

```
docker run -it -v ${PWD}/static:/static -p 8080:8080 touilleio/http-server-static-files:v1
```

## Configuration

The configuration can be set via environment variables, defined in the table below.

| Variable name | Default value | Details |
|---------------|---------------|-------------|
| `PORT`        | `8080`        | Port the http server listens on. |
| `ROOT_PATH`   | `/static`     | Path inside the container the http server is serving. Note that the process runs as `nobody`, folders and files in this path must be readable by `nobody`|

An example of changing the configuration via environment variable is provided below:

```
docker run -it -v ${PWD}/web:/web -e ROOT_PATH=/web -e PORT=8081 -p 8081:8081 touilleio/http-server-static-files:v1
```
