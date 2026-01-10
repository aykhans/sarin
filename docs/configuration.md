# Configuration

Sarin supports environment variables, CLI flags, and YAML files. However, they are not exactly equivalent—YAML files have the most configuration options, followed by CLI flags, and then environment variables.

When the same option is specified in multiple sources, the following priority order applies:

```
YAML (Highest) > CLI Flags > Environment Variables (Lowest)
```

Use `-s` or `--show-config` to see the final merged configuration before sending requests.

## Properties

> **Note:** For CLI flags with `string / []string` type, the flag can be used once with a single value or multiple times to provide multiple values.

| Name                        | YAML                                | CLI                                           | ENV                              | Default | Description                  |
| --------------------------- | ----------------------------------- | --------------------------------------------- | -------------------------------- | ------- | ---------------------------- |
| [Help](#help)               | -                                   | `-help` / `-h`                               | -                                | -       | Show help message            |
| [Version](#version)         | -                                   | `-version` / `-v`                            | -                                | -       | Show version and build info  |
| [Show Config](#show-config) | `showConfig`<br>(boolean)           | `-show-config` / `-s`<br>(boolean)           | `SARIN_SHOW_CONFIG`<br>(boolean) | `false` | Show merged configuration    |
| [Config File](#config-file) | `configFile`<br>(string / []string) | `-config-file` / `-f`<br>(string / []string) | `SARIN_CONFIG_FILE`<br>(string)  | -       | Path to config file(s)       |
| [URL](#url)                 | `url`<br>(string)                   | `-url` / `-U`<br>(string)                    | `SARIN_URL`<br>(string)          | -       | Target URL (HTTP/HTTPS)      |
| [Method](#method)           | `method`<br>(string / []string)     | `-method` / `-M`<br>(string / []string)      | `SARIN_METHOD`<br>(string)       | `GET`   | HTTP method(s)               |
| [Timeout](#timeout)         | `timeout`<br>(duration)             | `-timeout` / `-T`<br>(duration)              | `SARIN_TIMEOUT`<br>(duration)    | `10s`   | Request timeout              |
| [Concurrency](#concurrency) | `concurrency`<br>(number)           | `-concurrency` / `-c`<br>(number)            | `SARIN_CONCURRENCY`<br>(number)  | `1`     | Number of concurrent workers |
| [Requests](#requests)       | `requests`<br>(number)              | `-requests` / `-r`<br>(number)               | `SARIN_REQUESTS`<br>(number)     | -       | Total requests to send       |
| [Duration](#duration)       | `duration`<br>(duration)            | `-duration` / `-d`<br>(duration)             | `SARIN_DURATION`<br>(duration)   | -       | Test duration                |
| [Quiet](#quiet)             | `quiet`<br>(boolean)                | `-quiet` / `-q`<br>(boolean)                 | `SARIN_QUIET`<br>(boolean)       | `false` | Hide progress bar and logs   |
| [Output](#output)           | `output`<br>(string)                | `-output` / `-o`<br>(string)                 | `SARIN_OUTPUT`<br>(string)       | `table` | Output format for stats      |
| [Dry Run](#dry-run)         | `dryRun`<br>(boolean)               | `-dry-run` / `-z`<br>(boolean)               | `SARIN_DRY_RUN`<br>(boolean)     | `false` | Generate without sending     |
| [Insecure](#insecure)       | `insecure`<br>(boolean)             | `-insecure` / `-I`<br>(boolean)              | `SARIN_INSECURE`<br>(boolean)    | `false` | Skip TLS verification        |
| [Body](#body)               | `body`<br>(string / []string)       | `-body` / `-B`<br>(string / []string)        | `SARIN_BODY`<br>(string)         | -       | Request body                 |
| [Params](#params)           | `params`<br>(object)                | `-param` / `-P`<br>(string / []string)       | `SARIN_PARAM`<br>(string)        | -       | URL query parameters         |
| [Headers](#headers)         | `headers`<br>(object)               | `-header` / `-H`<br>(string / []string)      | `SARIN_HEADER`<br>(string)       | -       | HTTP headers                 |
| [Cookies](#cookies)         | `cookies`<br>(object)               | `-cookie` / `-C`<br>(string / []string)      | `SARIN_COOKIE`<br>(string)       | -       | HTTP cookies                 |
| [Proxy](#proxy)             | `proxy`<br>(string / []string)      | `-proxy` / `-X`<br>(string / []string)       | `SARIN_PROXY`<br>(string)        | -       | Proxy URL(s)                 |
| [Values](#values)           | `values`<br>(string / []string)     | `-values` / `-V`<br>(string / []string)      | `SARIN_VALUES`<br>(string)       | -       | Template values (key=value)  |

---

## Help

Show help message.

## Version

Show version and build information.

## Show Config

Show the final merged configuration before sending requests.

## Config File

Path to configuration file(s). Supports local paths and remote URLs.

If multiple config files are specified, they are merged in order. Later files override earlier ones.

**Example:**

```yaml
# config2.yaml
configFile: /config4.yaml
```

```sh
SARIN_CONFIG_FILE=/config1.yaml sarin -f /config2.yaml -f https://example.com/config3.yaml
```

In this example, all 4 config files are read and merged with the following priority:

```
config3.yaml > config2.yaml > config4.yaml > config1.yaml
```

## URL

Target URL. Must be HTTP or HTTPS.

## Method

HTTP method(s). If multiple values are provided, Sarin cycles through them randomly for each request. Supports [templating](templating.md).

**YAML example:**

```yaml
method: GET

# OR

method:
  - GET
  - POST
  - PUT
```

**CLI example:**

```sh
-method GET -method POST -method PUT
```

**ENV example:**

```sh
SARIN_METHOD=GET
```

## Timeout

Request timeout. Must be greater than 0.

Valid time units: `ns`, `us` (or `µs`), `ms`, `s`, `m`, `h`

**Examples:** `5s`, `300ms`, `1m20s`

## Concurrency

Number of concurrent workers. Must be between 1 and 100,000,000.

## Requests

Total number of requests to send. At least one of `requests` or `duration` must be specified. If both are provided, the test stops when either limit is reached first.

## Duration

Test duration. At least one of `requests` or `duration` must be specified. If both are provided, the test stops when either limit is reached first.

Valid time units: `ns`, `us` (or `µs`), `ms`, `s`, `m`, `h`

**Examples:** `1m30s`, `25s`, `1h`

## Quiet

Hide the progress bar and runtime logs.

## Output

Output format for response statistics.

Valid formats: `table`, `json`, `yaml`, `none`

Using `none` disables output and reduces memory usage since response statistics are not stored.

## Dry Run

Generate requests without sending them. Useful for testing templates.

## Insecure

Skip TLS certificate verification.

## Body

Request body. If multiple values are provided, Sarin cycles through them randomly for each request. Supports [templating](templating.md).

**YAML example:**

```yaml
body: '{"product": "car"}'

# OR

body:
  - '{"product": "car"}'
  - '{"product": "phone"}'
  - '{"product": "watch"}'
```

**CLI example:**

```sh
-body '{"product": "car"}' -body '{"product": "phone"}' -body '{"product": "watch"}'
```

**ENV example:**

```sh
SARIN_BODY='{"product": "car"}'
```

## Params

URL query parameters. If multiple values are provided for a key, Sarin cycles through them randomly for each request. Supports [templating](templating.md).

**YAML example:**

```yaml
params:
  key1: value1
  key2: [value2, value3]

# OR

params:
  - key1: value1
  - key2: [value2, value3]
```

**CLI example:**

```sh
-param "key1=value1" -param "key2=value2" -param "key2=value3"
```

**ENV example:**

```sh
SARIN_PARAM="key1=value1"
```

## Headers

HTTP headers. If multiple values are provided for a key, Sarin cycles through them randomly for each request. Supports [templating](templating.md).

**YAML example:**

```yaml
headers:
  key1: value1
  key2: [value2, value3]

# OR

headers:
  - key1: value1
  - key2: [value2, value3]
```

**CLI example:**

```sh
-header "key1: value1" -header "key2: value2" -header "key2: value3"
```

**ENV example:**

```sh
SARIN_HEADER="key1: value1"
```

## Cookies

HTTP cookies. If multiple values are provided for a key, Sarin cycles through them randomly for each request. Supports [templating](templating.md).

**YAML example:**

```yaml
cookies:
  key1: value1
  key2: [value2, value3]

# OR

cookies:
  - key1: value1
  - key2: [value2, value3]
```

**CLI example:**

```sh
-cookie "key1=value1" -cookie "key2=value2" -cookie "key2=value3"
```

**ENV example:**

```sh
SARIN_COOKIE="key1=value1"
```

## Proxy

Proxy URL(s). If multiple values are provided, Sarin cycles through them randomly for each request.

Supported protocols: `http`, `https`, `socks5`, `socks5h`

**YAML example:**

```yaml
proxy: http://proxy1.com

# OR

proxy:
  - http://proxy1.com
  - socks5://proxy2.com
  - socks5h://proxy3.com
```

**CLI example:**

```sh
-proxy http://proxy1.com -proxy socks5://proxy2.com -proxy socks5h://proxy3.com
```

**ENV example:**

```sh
SARIN_PROXY="http://proxy1.com"
```

## Values

Template values in key=value format. Supports [templating](templating.md). Multiple values can be specified and all are rendered for each request.

See the [Templating Guide](templating.md) for more details on using values and available template functions.

**YAML example:**

```yaml
values: "key=value"

# OR

values: |
  key1=value1
  key2=value2
  key3=value3
```

**CLI example:**

```sh
-values "key1=value1" -values "key2=value2" -values "key3=value3"
```

**ENV example:**

```sh
SARIN_VALUES="key1=value1"
```
