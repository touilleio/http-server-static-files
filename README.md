Http server serving static files
====

This project aims at providing a basic http server in a docker container for serving static files.
It is available for amd64, arm64 and arm/v7 platforms.

The latest image is published to GitHub Container Registry on every push to `main`:

```sh
docker pull ghcr.io/touilleio/http-server-static-files:latest
```

The workflow also publishes the same image under its full Git commit SHA. Prefer the SHA tag when a deployment must not follow future `latest` updates.

# Usage

```sh
docker run --rm -v "${PWD}/static:/static:ro" -p 8080:8080 ghcr.io/touilleio/http-server-static-files:latest
```

> [!WARNING]
> Every readable file and directory under `ROOT_PATH` is public, including dotfiles and dot-directories such as `.env` and `.git`. Directory listings are also generated when a directory has no `index.html`. Only mount a directory that contains deliberately public content. `os.OpenRoot` prevents symlinks from escaping `ROOT_PATH`, but it does not hide entries inside that root.

## Configuration

The configuration can be set via environment variables, defined in the table below.

| Variable name | Default value | Details |
|---------------|---------------|---------|
| `PORT` | `8080` | Port the HTTP server listens on. |
| `ROOT_PATH` | `/static` | Directory to publish. Files must be readable by the non-root user with UID `65532`. |
| `READ_HEADER_TIMEOUT` | `1s` | Maximum time to read request headers. |
| `READ_TIMEOUT` | `1s` | Maximum time to read an entire request. |
| `WRITE_TIMEOUT` | `1s` | Maximum response write duration. Increase this for large files or slow clients. |
| `IDLE_TIMEOUT` | `1s` | Maximum wait for the next keep-alive request. |
| `CONTENT_SECURITY_POLICY` | `default-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'` | Content Security Policy. Set an empty value to omit it. |
| `REFERRER_POLICY` | `strict-origin-when-cross-origin` | Referrer policy. Set an empty value to omit it. |
| `PERMISSIONS_POLICY` | `accelerometer=(), camera=(), geolocation=(), gyroscope=(), microphone=(), payment=(), usb=()` | Browser permissions policy. Set an empty value to omit it. |
| `FRAME_OPTIONS` | `DENY` | Value for `X-Frame-Options`. Set an empty value to omit it. |
| `STRICT_TRANSPORT_SECURITY` | empty | Optional HSTS value. Configure this only when the public endpoint is HTTPS. |

`X-Content-Type-Options: nosniff` is always set. Only `GET` and `HEAD` requests are served; other methods receive `405 Method Not Allowed`.

An example of changing the configuration via environment variable is provided below:

```sh
docker run --rm -v "${PWD}/web:/web:ro" -e ROOT_PATH=/web -e PORT=8081 -p 8081:8081 ghcr.io/touilleio/http-server-static-files:latest
```
