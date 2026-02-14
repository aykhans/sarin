# Examples

This guide provides practical examples for common Sarin use cases.

## Table of Contents

- [Basic Usage](#basic-usage)
- [Request-Based vs Duration-Based Tests](#request-based-vs-duration-based-tests)
- [Headers, Cookies, and Parameters](#headers-cookies-and-parameters)
- [Dynamic Requests with Templating](#dynamic-requests-with-templating)
- [Request Bodies](#request-bodies)
- [File Uploads](#file-uploads)
- [Using Proxies](#using-proxies)
- [Output Formats](#output-formats)
- [Docker Usage](#docker-usage)
- [Dry Run Mode](#dry-run-mode)
- [Show Configuration](#show-configuration)
- [Scripting](#scripting)

---

## Basic Usage

Send 1000 GET requests with 10 concurrent workers:

```sh
sarin -U http://example.com -r 1000 -c 10
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com
requests: 1000
concurrency: 10
```

</details>

Send requests with a custom timeout:

```sh
sarin -U http://example.com -r 1000 -c 10 -T 5s
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com
requests: 1000
concurrency: 10
timeout: 5s
```

</details>

## Request-Based vs Duration-Based Tests

**Request-based:** Stop after sending a specific number of requests:

```sh
sarin -U http://example.com -r 10000 -c 50
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com
requests: 10000
concurrency: 50
```

</details>

**Duration-based:** Run for a specific amount of time:

```sh
sarin -U http://example.com -d 5m -c 50
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com
duration: 5m
concurrency: 50
```

</details>

**Combined:** Stop when either limit is reached first:

```sh
sarin -U http://example.com -r 100000 -d 2m -c 100
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com
requests: 100000
duration: 2m
concurrency: 100
```

</details>

## Headers, Cookies, and Parameters

**Custom headers:**

```sh
sarin -U http://example.com -r 1000 -c 10 \
  -H "Authorization: Bearer token123" \
  -H "X-Custom-Header: value"
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com
requests: 1000
concurrency: 10
headers:
    Authorization: Bearer token123
    X-Custom-Header: value
```

</details>

**Random headers from multiple values:**

> **Note:** When multiple values are provided for the same header, Sarin starts at a random index and cycles through all values in order. Once the cycle completes, it picks a new random starting point. This ensures all values are used while maintaining some randomness.

```sh
sarin -U http://example.com -r 1000 -c 10 \
  -H "X-Region: us-east" \
  -H "X-Region: us-west" \
  -H "X-Region: eu-central"
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com
requests: 1000
concurrency: 10
headers:
    X-Region:
        - us-east
        - us-west
        - eu-central
```

</details>

**Query parameters:**

```sh
sarin -U http://example.com/search -r 1000 -c 10 \
  -P "query=test" \
  -P "limit=10"
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com/search
requests: 1000
concurrency: 10
params:
    query: "test"
    limit: 10
```

</details>

**Dynamic query parameters:**

```sh
sarin -U http://example.com/users -r 1000 -c 10 \
  -P "id={{ fakeit_IntRange 1 1000 }}" \
  -P "fields=name,email"
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com/users
requests: 1000
concurrency: 10
params:
    id: "{{ fakeit_IntRange 1 1000 }}"
    fields: "name,email"
```

</details>

**Cookies:**

```sh
sarin -U http://example.com -r 1000 -c 10 \
  -C "session_id=abc123" \
  -C "user_id={{ fakeit_UUID }}"
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com
requests: 1000
concurrency: 10
cookies:
    session_id: abc123
    user_id: "{{ fakeit_UUID }}"
```

</details>

## Dynamic Requests with Templating

**Dynamic URL paths:**

Test different resource endpoints with random IDs:

```sh
sarin -U "http://example.com/users/{{ fakeit_UUID }}/profile" -r 1000 -c 10
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com/users/{{ fakeit_UUID }}/profile
requests: 1000
concurrency: 10
```

</details>

Test with random numeric IDs:

```sh
sarin -U "http://example.com/products/{{ fakeit_Number 1 10000 }}" -r 1000 -c 10
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com/products/{{ fakeit_Number 1 10000 }}
requests: 1000
concurrency: 10
```

</details>

**Generate a random User-Agent for each request:**

```sh
sarin -U http://example.com -r 1000 -c 10 \
  -H "User-Agent: {{ fakeit_UserAgent }}"
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com
requests: 1000
concurrency: 10
headers:
    User-Agent: "{{ fakeit_UserAgent }}"
```

</details>

Send requests with random user data:

```sh
sarin -U http://example.com/api/users -r 1000 -c 10 \
  -M POST \
  -H "Content-Type: application/json" \
  -B '{"name": "{{ fakeit_Name }}", "email": "{{ fakeit_Email }}"}'
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com/api/users
requests: 1000
concurrency: 10
method: POST
headers:
    Content-Type: application/json
body: '{"name": "{{ fakeit_Name }}", "email": "{{ fakeit_Email }}"}'
```

</details>

Use values to share generated data across headers and body:

```sh
sarin -U http://example.com/api/users -r 1000 -c 10 \
  -M POST \
  -V "ID={{ fakeit_UUID }}" \
  -H "X-Request-ID: {{ .Values.ID }}" \
  -B '{"id": "{{ .Values.ID }}", "name": "{{ fakeit_Name }}"}'
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com/api/users
requests: 1000
concurrency: 10
method: POST
values: "ID={{ fakeit_UUID }}"
headers:
    X-Request-ID: "{{ .Values.ID }}"
body: '{"id": "{{ .Values.ID }}", "name": "{{ fakeit_Name }}"}'
```

</details>

Generate random IPs and timestamps:

```sh
sarin -U http://example.com/api/logs -r 500 -c 20 \
  -M POST \
  -H "Content-Type: application/json" \
  -B '{"ip": "{{ fakeit_IPv4Address }}", "timestamp": "{{ fakeit_Date }}", "action": "{{ fakeit_HackerVerb }}"}'
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com/api/logs
requests: 500
concurrency: 20
method: POST
headers:
    Content-Type: application/json
body: '{"ip": "{{ fakeit_IPv4Address }}", "timestamp": "{{ fakeit_Date }}", "action": "{{ fakeit_HackerVerb }}"}'
```

</details>

> For the complete list of 320+ template functions, see the **[Templating Guide](templating.md)**.

## Request Bodies

**Simple JSON body:**

```sh
sarin -U http://example.com/api/data -r 1000 -c 10 \
  -M POST \
  -H "Content-Type: application/json" \
  -B '{"key": "value"}'
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com/api/data
requests: 1000
concurrency: 10
method: POST
headers:
    Content-Type: application/json
body: '{"key": "value"}'
```

</details>

**Multiple bodies (randomly cycled):**

```sh
sarin -U http://example.com/api/products -r 1000 -c 10 \
  -M POST \
  -H "Content-Type: application/json" \
  -B '{"product": "laptop", "price": 999}' \
  -B '{"product": "phone", "price": 599}' \
  -B '{"product": "tablet", "price": 399}'
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com/api/products
requests: 1000
concurrency: 10
method: POST
headers:
    Content-Type: application/json
body:
    - '{"product": "laptop", "price": 999}'
    - '{"product": "phone", "price": 599}'
    - '{"product": "tablet", "price": 399}'
```

</details>

**Dynamic body with fake data:**

```sh
sarin -U http://example.com/api/orders -r 1000 -c 10 \
  -M POST \
  -H "Content-Type: application/json" \
  -B '{
    "order_id": "{{ fakeit_UUID }}",
    "customer": "{{ fakeit_Name }}",
    "email": "{{ fakeit_Email }}",
    "amount": {{ fakeit_Price 10 500 }},
    "currency": "{{ fakeit_CurrencyShort }}"
  }'
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com/api/orders
requests: 1000
concurrency: 10
method: POST
headers:
    Content-Type: application/json
body: |
    {
      "order_id": "{{ fakeit_UUID }}",
      "customer": "{{ fakeit_Name }}",
      "email": "{{ fakeit_Email }}",
      "amount": {{ fakeit_Price 10 500 }},
      "currency": "{{ fakeit_CurrencyShort }}"
    }
```

</details>

**Multipart form data:**

```sh
sarin -U http://example.com/api/upload -r 1000 -c 10 \
  -M POST \
  -B '{{ body_FormData "username" "john" "email" "john@example.com" }}'
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com/api/upload
requests: 1000
concurrency: 10
method: POST
body: '{{ body_FormData "username" "john" "email" "john@example.com" }}'
```

</details>

**Multipart form data with dynamic values:**

```sh
sarin -U http://example.com/api/users -r 1000 -c 10 \
  -M POST \
  -B '{{ body_FormData "name" (fakeit_Name) "email" (fakeit_Email) "phone" (fakeit_Phone) }}'
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com/api/users
requests: 1000
concurrency: 10
method: POST
body: '{{ body_FormData "name" (fakeit_Name) "email" (fakeit_Email) "phone" (fakeit_Phone) }}'
```

</details>

> **Note:** `body_FormData` automatically sets the `Content-Type` header to `multipart/form-data` with the appropriate boundary.

## File Uploads

**File upload with multipart form data:**

Upload a local file:

```sh
sarin -U http://example.com/api/upload -r 100 -c 10 \
  -M POST \
  -B '{{ body_FormData "title" "My Document" "document" "@/path/to/file.pdf" }}'
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com/api/upload
requests: 100
concurrency: 10
method: POST
body: '{{ body_FormData "title" "My Document" "document" "@/path/to/file.pdf" }}'
```

</details>

**Multiple file uploads (same field name):**

```sh
sarin -U http://example.com/api/upload -r 100 -c 10 \
  -M POST \
  -B '{{ body_FormData "files" "@/path/to/file1.pdf" "files" "@/path/to/file2.pdf" }}'
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com/api/upload
requests: 100
concurrency: 10
method: POST
body: |
    {{ body_FormData
       "files" "@/path/to/file1.pdf"
       "files" "@/path/to/file2.pdf"
    }}
```

</details>

**Multiple file uploads (different field names):**

```sh
sarin -U http://example.com/api/upload -r 100 -c 10 \
  -M POST \
  -B '{{ body_FormData "avatar" "@/path/to/photo.jpg" "resume" "@/path/to/cv.pdf" "cover_letter" "@/path/to/letter.docx" }}'
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com/api/upload
requests: 100
concurrency: 10
method: POST
body: |
    {{ body_FormData
       "avatar" "@/path/to/photo.jpg"
       "resume" "@/path/to/cv.pdf"
       "cover_letter" "@/path/to/letter.docx"
    }}
```

</details>

**File from URL:**

```sh
sarin -U http://example.com/api/upload -r 100 -c 10 \
  -M POST \
  -B '{{ body_FormData "image" "@https://example.com/photo.jpg" }}'
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com/api/upload
requests: 100
concurrency: 10
method: POST
body: '{{ body_FormData "image" "@https://example.com/photo.jpg" }}'
```

</details>

> **Note:** Files (local and remote) are cached in memory after the first read, so they are not re-read for every request.

**Base64 encoded file in JSON body (local file):**

```sh
sarin -U http://example.com/api/upload -r 100 -c 10 \
  -M POST \
  -H "Content-Type: application/json" \
  -B '{"file": "{{ file_Base64 "/path/to/file.pdf" }}", "filename": "document.pdf"}'
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com/api/upload
requests: 100
concurrency: 10
method: POST
headers:
    Content-Type: application/json
body: '{"file": "{{ file_Base64 "/path/to/file.pdf" }}", "filename": "document.pdf"}'
```

</details>

**Base64 encoded file in JSON body (remote URL):**

```sh
sarin -U http://example.com/api/upload -r 100 -c 10 \
  -M POST \
  -H "Content-Type: application/json" \
  -B '{"image": "{{ file_Base64 "https://example.com/photo.jpg" }}", "filename": "photo.jpg"}'
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com/api/upload
requests: 100
concurrency: 10
method: POST
headers:
    Content-Type: application/json
body: '{"image": "{{ file_Base64 "https://example.com/photo.jpg" }}", "filename": "photo.jpg"}'
```

</details>

## Using Proxies

**Single HTTP proxy:**

```sh
sarin -U http://example.com -r 1000 -c 10 \
  -X http://proxy.example.com:8080
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com
requests: 1000
concurrency: 10
proxy: http://proxy.example.com:8080
```

</details>

**SOCKS5 proxy:**

```sh
sarin -U http://example.com -r 1000 -c 10 \
  -X socks5://proxy.example.com:1080
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com
requests: 1000
concurrency: 10
proxy: socks5://proxy.example.com:1080
```

</details>

**Multiple proxies (load balanced):**

```sh
sarin -U http://example.com -r 1000 -c 10 \
  -X http://proxy1.example.com:8080 \
  -X http://proxy2.example.com:8080 \
  -X socks5://proxy3.example.com:1080
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com
requests: 1000
concurrency: 10
proxy:
    - http://proxy1.example.com:8080
    - http://proxy2.example.com:8080
    - socks5://proxy3.example.com:1080
```

</details>

**Proxy with authentication:**

```sh
sarin -U http://example.com -r 1000 -c 10 \
  -X http://user:password@proxy.example.com:8080
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com
requests: 1000
concurrency: 10
proxy: http://user:password@proxy.example.com:8080
```

</details>

## Output Formats

**Table output (default):**

```sh
sarin -U http://example.com -r 1000 -c 10 -o table
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com
requests: 1000
concurrency: 10
output: table
```

</details>

**JSON output (useful for parsing):**

```sh
sarin -U http://example.com -r 1000 -c 10 -o json
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com
requests: 1000
concurrency: 10
output: json
```

</details>

**YAML output:**

```sh
sarin -U http://example.com -r 1000 -c 10 -o yaml
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com
requests: 1000
concurrency: 10
output: yaml
```

</details>

**No output (minimal memory usage):**

```sh
sarin -U http://example.com -r 1000 -c 10 -o none
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com
requests: 1000
concurrency: 10
output: none
```

</details>

**Quiet mode (hide progress bar):**

```sh
sarin -U http://example.com -r 1000 -c 10 -q
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com
requests: 1000
concurrency: 10
quiet: true
```

</details>

## Docker Usage

**Basic Docker usage:**

```sh
docker run -it --rm aykhans/sarin -U http://example.com -r 1000 -c 10
```

**With local config file:**

```sh
docker run -it --rm -v $(pwd)/config.yaml:/config.yaml aykhans/sarin -f /config.yaml
```

**With remote config file:**

```sh
docker run -it --rm aykhans/sarin -f https://example.com/config.yaml
```

**Interactive mode with TTY:**

```sh
docker run --rm -it aykhans/sarin -U http://example.com -r 1000 -c 10
```

## Dry Run Mode

Test your configuration without sending actual requests:

```sh
sarin -U http://example.com -r 10 -c 1 -z \
  -H "X-Request-ID: {{ fakeit_UUID }}" \
  -B '{"user": "{{ fakeit_Name }}"}'
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com
requests: 10
concurrency: 1
dryRun: true
headers:
    X-Request-ID: "{{ fakeit_UUID }}"
body: '{"user": "{{ fakeit_Name }}"}'
```

</details>

This validates templates.

## Show Configuration

Preview the merged configuration before running:

```sh
sarin -U http://example.com -r 1000 -c 10 \
  -H "Authorization: Bearer token" \
  -s
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com
requests: 1000
concurrency: 10
showConfig: true
headers:
    Authorization: Bearer token
```

</details>

## Scripting

Transform requests using Lua or JavaScript scripts. Scripts run after template rendering, before the request is sent.

**Add a custom header with Lua:**

```sh
sarin -U http://example.com/api -r 1000 -c 10 \
  -lua 'function transform(req) req.headers["X-Custom"] = "my-value" return req end'
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com/api
requests: 1000
concurrency: 10
lua: |
    function transform(req)
        req.headers["X-Custom"] = "my-value"
        return req
    end
```

</details>

**Modify request body with JavaScript:**

```sh
sarin -U http://example.com/api/data -r 1000 -c 10 \
  -M POST \
  -H "Content-Type: application/json" \
  -B '{"name": "test"}' \
  -js 'function transform(req) { var body = JSON.parse(req.body); body.timestamp = Date.now(); req.body = JSON.stringify(body); return req; }'
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com/api/data
requests: 1000
concurrency: 10
method: POST
headers:
    Content-Type: application/json
body: '{"name": "test"}'
js: |
    function transform(req) {
        var body = JSON.parse(req.body);
        body.timestamp = Date.now();
        req.body = JSON.stringify(body);
        return req;
    }
```

</details>

**Load script from a file:**

```sh
sarin -U http://example.com/api -r 1000 -c 10 \
  -lua @./scripts/transform.lua
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com/api
requests: 1000
concurrency: 10
lua: "@./scripts/transform.lua"
```

</details>

**Load script from a URL:**

```sh
sarin -U http://example.com/api -r 1000 -c 10 \
  -js @https://example.com/scripts/transform.js
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com/api
requests: 1000
concurrency: 10
js: "@https://example.com/scripts/transform.js"
```

</details>

**Chain multiple scripts (Lua runs first, then JavaScript):**

```sh
sarin -U http://example.com/api -r 1000 -c 10 \
  -lua @./scripts/auth.lua \
  -lua @./scripts/headers.lua \
  -js @./scripts/body.js
```

<details>
<summary>YAML equivalent</summary>

```yaml
url: http://example.com/api
requests: 1000
concurrency: 10
lua:
    - "@./scripts/auth.lua"
    - "@./scripts/headers.lua"
js: "@./scripts/body.js"
```

</details>
