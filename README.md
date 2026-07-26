<div align="center">

## Sarin is a high-performance HTTP load testing tool built with Go and fasthttp.

[![Go Reference](https://pkg.go.dev/badge/go.aykhans.me/sarin.svg)](https://pkg.go.dev/go.aykhans.me/sarin)
[![Docker Pulls](https://img.shields.io/docker/pulls/aykhans/sarin)](https://hub.docker.com/r/aykhans/sarin)
[![Downloads](https://img.shields.io/github/downloads/aykhans/sarin/total?cacheSeconds=3600)](https://github.com/aykhans/sarin/releases)
[![Lint](https://img.shields.io/github/actions/workflow/status/aykhans/sarin/lint.yaml?branch=main&label=lint)](https://github.com/aykhans/sarin/actions/workflows/lint.yaml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

</div>

![Demo](docs/static/demo.gif)

<p align="center">
  <a href="#installation">Install</a> •
  <a href="#quick-start">Quick Start</a> •
  <a href="docs/examples.md">Examples</a> •
  <a href="docs/configuration.md">Configuration</a> •
  <a href="docs/templating.md">Templating</a>
</p>

## Overview

Sarin is designed for efficient HTTP load testing with minimal resource consumption. It prioritizes simplicity and features like templating add zero overhead when unused.

| ✅ Supported                                               | ❌ Not Supported                |
| ---------------------------------------------------------- | ------------------------------- |
| High-performance with low memory footprint                 | Detailed response body analysis |
| Long-running duration/count based tests                    | Extensive response statistics   |
| Dynamic requests via 340+ template functions               | Web UI or complex TUI           |
| Request scripting with Lua and JavaScript                  | Distributed load testing        |
| Multiple proxy protocols<br>(HTTP, HTTPS, SOCKS5, SOCKS5H) | HTTP/2, HTTP/3, WebSocket, gRPC |
| Captcha solving<br>(2Captcha, Anti-Captcha, CapSolver)     | Plugins / extensions ecosystem  |
| Flexible config (CLI, ENV, YAML)                           |                                 |

## Installation

<details open>
<summary><b>Docker</b></summary>

```sh
docker run -it aykhans/sarin:latest --version
```

With a local config file:

```sh
docker run --rm -it -v /path/to/config.yaml:/config.yaml aykhans/sarin -f /config.yaml
```

With a remote config file:

```sh
docker run --rm -it aykhans/sarin -f https://example.com/config.yaml
```

</details>

<details>
<summary><b>Nix</b></summary>

Run directly without installing (requires flakes enabled):

```sh
nix run github:aykhans/sarin/v1.3.2 -- -U http://example.com -r 100 -c 10
```

Install into your profile:

```sh
nix profile install github:aykhans/sarin/v1.3.2
```

Or add it to your own flake via the overlay:

```nix
{
  inputs.sarin.url = "github:aykhans/sarin/v1.3.2";

  # In your outputs, apply the overlay to nixpkgs:
  # nixpkgs.overlays = [ inputs.sarin.overlays.default ];
  # then reference pkgs.sarin
}
```

</details>

<details>
<summary><b>Pre-built Binaries</b></summary>

Download the latest binaries from the [releases](https://github.com/aykhans/sarin/releases) page.

</details>

<details>
<summary><b>Building from Source</b></summary>

Requires [Go 1.26+](https://golang.org/dl/).

```sh
git clone https://github.com/aykhans/sarin.git && cd sarin

CGO_ENABLED=0 go build \
    -ldflags "-X 'go.aykhans.me/sarin/internal/version.Version=dev' \
    -X 'go.aykhans.me/sarin/internal/version.GitCommit=$(git rev-parse HEAD)' \
    -X 'go.aykhans.me/sarin/internal/version.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)' \
    -X 'go.aykhans.me/sarin/internal/version.GoVersion=$(go version)' \
    -s -w" \
    -o sarin ./cmd/cli/main.go
```

</details>

## Quick Start

Send 10,000 GET requests with 50 concurrent connections and a random User-Agent for each request:

```sh
sarin -U http://example.com -r 10_000 -c 50 -H "User-Agent: {{ fakeit_UserAgent }}"
```

Run a 5-minute duration-based test:

```sh
sarin -U http://example.com -d 5m -c 100
```

Use a YAML config file:

```sh
sarin -f config.yaml
```

For more usage examples, see the **[Examples Guide](docs/examples.md)**.

## Configuration

Sarin supports environment variables, CLI flags, and YAML files. When the same option is specified in multiple sources, the following priority order applies:

```
CLI Flags (Highest) > YAML > Environment Variables (Lowest)
```

For detailed documentation on all configuration options (URL, method, timeout, concurrency, headers, cookies, proxy, etc.), see the **[Configuration Guide](docs/configuration.md)**.

## Templating

Sarin supports Go templates in URL paths, methods, bodies, headers, params, cookies, and values. Use the 340+ built-in functions to generate dynamic data for each request.

**Example:**

```sh
sarin -U "http://example.com/users/{{ fakeit_UUID }}" -r 1000 -c 10 \
  -V "REQUEST_ID={{ fakeit_UUID }}" \
  -H "X-Request-ID: {{ .Values.REQUEST_ID }}" \
  -B '{"request_id": "{{ .Values.REQUEST_ID }}"}'
```

For the complete templating guide and functions reference, see the **[Templating Guide](docs/templating.md)**.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
