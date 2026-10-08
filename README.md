# mautrix-linkedin
A Matrix-LinkedIn puppeting bridge.

## Documentation
All setup and usage instructions are located on [docs.mau.fi]. Some quick links:

[docs.mau.fi]: https://docs.mau.fi/bridges/go/linkedin/index.html

* [Bridge setup](https://docs.mau.fi/bridges/go/setup.html?bridge=linkedin)
  (or [with Docker](https://docs.mau.fi/bridges/general/docker-setup.html?bridge=linkedin))
* Basic usage: [Authentication](https://docs.mau.fi/bridges/go/linkedin/authentication.html)

### Features & Roadmap

[ROADMAP.md](ROADMAP.md) contains a general overview of what is supported by the bridge.

### Native login diagnostics

Native login supports email/password and the observed six-digit email verification
challenge. Other verification types offer browser login until their request and
completion flows are supported.

To debug an unfamiliar challenge, set `network.log_redacted_login_responses: true`
and enable debug logging. Native login responses, including HTTP errors, then
include a `response_redacted_gz` field: base64-encoded gzip containing redacted
HTML, JSON, or Flight rows (hydrated pages are logged as their Flight rows), with
`response_format` saying which. HTML form names and SDUI action
structure are retained. Private values and arbitrary scripts use process-keyed
redaction markers; free-form prose is also redacted. Requests, cookies, and feed
bodies are excluded. Partial responses, bodies over 1 MiB, and encoded diagnostics
over 64 KiB include an omission reason instead of a body.

Decode one event from a JSON log with:

```sh
jq -r 'select(.response_redacted_gz) | .response_redacted_gz' bridge.log | head -n 1 | base64 --decode | gzip --decompress
```

The setting defaults to false and replaces the temporary
`LINKEDIN_LOGIN_RESPONSE_DIR` raw-capture option. Redacted diagnostics help identify
unsupported flows; they cannot replay a verification session.

## Discussion
Matrix room: [#linkedin:maunium.net](https://matrix.to/#/#linkedin:maunium.net)
