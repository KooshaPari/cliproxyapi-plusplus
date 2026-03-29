# User Journeys — CLIProxyAPI Plus

---

## UJ-1: Developer Using OpenAI SDK Against Anthropic Claude

**Actor**: Developer
**Goal**: Use existing OpenAI Python/TypeScript code against Claude without changing client code.

```
Developer                CLIProxyAPI Plus              Anthropic API
    |                          |                             |
    |-- POST /v1/chat/completions (OpenAI format) -->        |
    |   { model: "claude-3-7-sonnet", messages: [...] }      |
    |                          |                             |
    |                    [lookup provider by model]          |
    |                    [translate request to Anthropic format]
    |                          |-- POST /v1/messages ------> |
    |                          |   { model, messages, ... }  |
    |                          |                             |
    |                          |<-- 200 Anthropic response --+
    |                    [translate response to OpenAI format]
    |<-- 200 OpenAI-format response ----------------         |
    |   { choices: [{ message: { role, content } }] }        |
```

**Steps**:
1. Developer sets `OPENAI_BASE_URL=http://localhost:8080` and `OPENAI_API_KEY=<proxy-key>`.
2. Developer calls `openai.chat.completions.create({ model: "claude-3-7-sonnet", ... })`.
3. CLIProxyAPI Plus receives request, routes to Anthropic translator.
4. Anthropic response translated back to OpenAI format.
5. Developer receives response identical in structure to OpenAI response.

**Failure Paths**:
- Anthropic API key invalid: HTTP 401 returned with clear error.
- Model not found in registry: HTTP 400 `model not supported`.

---

## UJ-2: Operator Configuring a New Provider

**Actor**: Operator (self-hosted deployment)
**Goal**: Add a new API key and enable a provider without restarting the server.

```
Operator                  config.yaml            CLIProxyAPI Plus
    |                          |                        |
    |-- edit config.yaml ----> |                        |
    |   providers:             |                        |
    |     gemini:              |                        |
    |       api_key: "..."     |                        |
    |                          |-- fsnotify event ----> |
    |                          |                  [re-register providers]
    |                          |                  [new Gemini translator live]
    |-- POST /v1/chat/completions (gemini model) ------> |
    |<-- 200 success ----------------------------------- |
```

**Steps**:
1. Operator edits `config.yaml`, adds Gemini API key.
2. fsnotify detects file change; registry reloads providers.
3. In-flight requests complete against old config.
4. New requests to Gemini models route correctly.

---

## UJ-3: Developer Using the Go SDK

**Actor**: Go developer integrating CLIProxyAPI programmatically

```
Go Code                  CLIProxyAPI SDK            CLIProxyAPI Server
    |                          |                          |
    |-- sdk.NewClient(cfg) --> |                          |
    |                    [configure base URL, auth]       |
    |-- client.Complete(req) ->|                          |
    |                    [translate to HTTP request]      |
    |                          |-- POST /v1/chat/completions -->
    |                          |<-- 200 response ----------|
    |                    [parse to typed Go struct]        |
    |<-- CompletionResponse -- |                          |
```

**Steps**:
1. Developer imports `sdk` package.
2. Calls `sdk.NewClient` with base URL and auth config.
3. Calls `client.Complete(ctx, req)` with typed request struct.
4. Receives typed `CompletionResponse`.
5. Streaming variant: calls `client.Stream(ctx, req)` receiving `<-chan Event`.

---

## UJ-4: Operator Monitoring via TUI Dashboard

**Actor**: Operator running CLIProxyAPI Plus in a terminal

```
Terminal
  +------------------------------------------+
  | CLIProxyAPI Plus Dashboard               |
  +------------------------------------------+
  | Active Requests: 12                      |
  | Provider Breakdown:                      |
  |   claude-3-7-sonnet  ########  8 reqs   |
  |   gemini-2.0-flash   ###       3 reqs   |
  |   cursor             #         1 req    |
  | Token Usage (last 5m): 142,300          |
  | Error Rate: 0.0%                        |
  +------------------------------------------+
```

**Steps**:
1. Operator starts server with `--tui` flag.
2. TUI renders in terminal with real-time updates.
3. Operator observes active request counts by provider.
4. On error spike, operator checks logs via log panel.

---

## UJ-5: Community Contributor Adding a Provider

**Actor**: Open-source contributor
**Goal**: Add support for a new third-party AI provider.

```
Contributor              internal/interfaces        internal/registry
    |                          |                          |
    |-- implement Translator -->|                          |
    |   type MyProvider struct  |                          |
    |   func Translate(req) ... |                          |
    |                          |                          |
    |-- register in registry ------------------>           |
    |   registry.Register("myprovider", &MyProvider{})    |
    |                          |                          |
    |-- add config example --------------------------->    |
    |   config.example.yaml: providers.myprovider.api_key |
    |                                                      |
    |-- submit PR ---------------------------------------->|
```

**Steps**:
1. Contributor creates new file in `auths/` or `internal/registry/`.
2. Implements `interfaces.Translator` interface.
3. Registers provider in registry init.
4. Adds config example entry.
5. Adds integration test.
6. Submits PR — only third-party provider changes accepted.
