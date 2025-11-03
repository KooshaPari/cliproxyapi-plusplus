# 管理 API

基础路径：`http://localhost:8317/v0/management`

该 API 用于管理 CLI Proxy API 的运行时配置与认证文件。所有变更会持久化写入 YAML 配置文件，并由服务自动热重载。

注意：以下选项不能通过 API 修改，需在配置文件中设置（如有必要可重启）：
- `allow-remote-management`
- `remote-management-key`（若在启动时检测到明文，会自动进行 bcrypt 加密并写回配置）

## 认证

- 所有请求（包括本地访问）都必须提供有效的管理密钥.
- 远程访问需要在配置文件中开启远程访问： `allow-remote-management: true`
- 通过以下任意方式提供管理密钥（明文）：
  - `Authorization: Bearer <plaintext-key>`
  - `X-Management-Key: <plaintext-key>`

若在启动时检测到配置中的管理密钥为明文，会自动使用 bcrypt 加密并回写到配置文件中。

其它说明：
- 若 `remote-management.secret-key` 为空，则管理 API 整体被禁用（所有 `/v0/management` 路由均返回 404）。
- 对于远程 IP，连续 5 次认证失败会触发临时封禁（约 30 分钟）。

## 请求/响应约定

- Content-Type：`application/json`（除非另有说明）。
- 布尔/整数/字符串更新：请求体为 `{ "value": <type> }`。
- 数组 PUT：既可使用原始数组（如 `["a","b"]`），也可使用 `{ "items": [ ... ] }`。
- 数组 PATCH：支持 `{ "old": "k1", "new": "k2" }` 或 `{ "index": 0, "value": "k2" }`。
- 对象数组 PATCH：支持按索引或按关键字段匹配（各端点中单独说明）。

## 端点说明

### Usage（请求统计）
- GET `/usage` — 获取内存中的请求统计
  - 响应：
    ```json
    {
      "usage": {
        "total_requests": 24,
        "success_count": 22,
        "failure_count": 2,
        "total_tokens": 13890,
        "requests_by_day": {
          "2024-05-20": 12
        },
        "requests_by_hour": {
          "09": 4,
          "18": 8
        },
        "tokens_by_day": {
          "2024-05-20": 9876
        },
        "tokens_by_hour": {
          "09": 1234,
          "18": 865
        },
        "apis": {
          "POST /v1/chat/completions": {
            "total_requests": 12,
            "total_tokens": 9021,
            "models": {
              "gpt-4o-mini": {
                "total_requests": 8,
                "total_tokens": 7123,
                "details": [
                  {
                    "timestamp": "2024-05-20T09:15:04.123456Z",
                    "tokens": {
                      "input_tokens": 523,
                      "output_tokens": 308,
                      "reasoning_tokens": 0,
                      "cached_tokens": 0,
                      "total_tokens": 831
                    }
                  }
                ]
              }
            }
          }
        }
      },
      "failed_requests": 2
    }
    ```
  - 说明：
    - 仅统计带有 token 使用信息的请求，服务重启后数据会被清空。
    - 小时维度会将所有日期折叠到 `00`–`23` 的统一小时桶中。
    - 顶层字段 `failed_requests` 与 `usage.failure_count` 相同，便于轮询。

### Config
- GET `/config` — 获取完整的配置
    - 请求:
      ```bash
      curl -H 'Authorization: Bearer <MANAGEMENT_KEY>' http://localhost:8317/v0/management/config
      ```
    - 响应:
      ```json
      {"debug":true,"proxy-url":"","api-keys":["1...5","JS...W"],"quota-exceeded":{"switch-project":true,"switch-preview-model":true},"gemini-api-key":[{"api-key":"AI...01","base-url":"https://generativelanguage.googleapis.com","headers":{"X-Custom-Header":"custom-value"},"proxy-url":""},{"api-key":"AI...02","proxy-url":"socks5://proxy.example.com:1080"}],"gl-api-key":["AI...01","AI...02"],"request-log":true,"request-retry":3,"claude-api-key":[{"api-key":"cr...56","base-url":"https://example.com/api","proxy-url":"socks5://proxy.example.com:1080","models":[{"name":"claude-3-5-sonnet-20241022","alias":"claude-sonnet-latest"}]},{"api-key":"cr...e3","base-url":"http://example.com:3000/api","proxy-url":""},{"api-key":"sk-...q2","base-url":"https://example.com","proxy-url":""}],"codex-api-key":[{"api-key":"sk...01","base-url":"https://example/v1","proxy-url":""}],"openai-compatibility":[{"name":"openrouter","base-url":"https://openrouter.ai/api/v1","api-key-entries":[{"api-key":"sk...01","proxy-url":""}],"models":[{"name":"moonshotai/kimi-k2:free","alias":"kimi-k2"}]},{"name":"iflow","base-url":"https://apis.iflow.cn/v1","api-key-entries":[{"api-key":"sk...7e","proxy-url":"socks5://proxy.example.com:1080"}],"models":[{"name":"deepseek-v3.1","alias":"deepseek-v3.1"},{"name":"glm-4.5","alias":"glm-4.5"},{"name":"kimi-k2","alias":"kimi-k2"}]}]}
      ```
  - 说明：
    - 返回中会附带 `gl-api-key`，其内容来自 `gemini-api-key` 的 API Key 列表（仅保留纯字符串视图）。
    - 若服务尚未加载配置文件，则返回空对象 `{}`。

### Debug
- GET `/debug` — 获取当前 debug 状态
  - 请求：
    ```bash
    curl -H 'Authorization: Bearer <MANAGEMENT_KEY>' http://localhost:8317/v0/management/debug
    ```
  - 响应：
    ```json
    { "debug": false }
    ```
- PUT/PATCH `/debug` — 设置 debug（布尔值）
  - 请求：
    ```bash
    curl -X PUT -H 'Content-Type: application/json' \
    -H 'Authorization: Bearer <MANAGEMENT_KEY>' \
      -d '{"value":true}' \
      http://localhost:8317/v0/management/debug
    ```
  - 响应：
    ```json
    { "status": "ok" }
    ```

### Config YAML
- GET `/config.yaml` — 原样下载持久化的 YAML 配置
  - 响应头：
    - `Content-Type: application/yaml; charset=utf-8`
    - `Cache-Control: no-store`
  - 响应体：保留注释与格式的原始 YAML 流。
- PUT `/config.yaml` — 使用 YAML 文档整体替换配置
  - 请求：
    ```bash
    curl -X PUT -H 'Content-Type: application/yaml' \
    -H 'Authorization: Bearer <MANAGEMENT_KEY>' \
      --data-binary @config.yaml \
      http://localhost:8317/v0/management/config.yaml
    ```
  - 响应：
    ```json
    { "ok": true, "changed": ["config"] }
    ```
  - 说明：
    - 服务会先加载 YAML 验证其有效性，校验失败返回 `422` 以及 `{ "error": "invalid_config", "message": "..." }`。
    - 写入失败会返回 `500`，格式 `{ "error": "write_failed", "message": "..." }`。

### 文件日志开关
- GET `/logging-to-file` — 查看是否启用文件日志
  - 响应：
    ```json
    { "logging-to-file": true }
    ```
- PUT/PATCH `/logging-to-file` — 开启或关闭文件日志
  - 请求：
    ```bash
    curl -X PATCH -H 'Content-Type: application/json' \
    -H 'Authorization: Bearer <MANAGEMENT_KEY>' \
      -d '{"value":false}' \
      http://localhost:8317/v0/management/logging-to-file
    ```
  - 响应：
    ```json
    { "status": "ok" }
    ```

### 日志文件
- GET `/logs` — 获取合并后的最新日志行
  - 查询参数：
    - `after`（可选）：Unix 时间戳，仅返回该时间之后的日志。
  - 响应：
    ```json
    {
      "lines": ["2024-05-20 12:00:00 info request accepted"],
      "line-count": 125,
      "latest-timestamp": 1716206400
    }
    ```
  - 说明：
    - 需要先启用文件日志，否则会以 `400` 返回 `{ "error": "logging to file disabled" }`。
    - 若当前没有日志文件，返回的 `lines` 为空数组、`line-count` 为 `0`。
- DELETE `/logs` — 删除轮换日志并清空主日志
  - 响应：
    ```json
    { "success": true, "message": "Logs cleared successfully", "removed": 3 }
    ```

### Usage 统计开关
- GET `/usage-statistics-enabled` — 查看是否启用请求统计
  - 响应：
    ```json
    { "usage-statistics-enabled": true }
    ```
- PUT/PATCH `/usage-statistics-enabled` — 启用或关闭统计
  - 请求：
    ```bash
    curl -X PUT -H 'Content-Type: application/json' \
    -H 'Authorization: Bearer <MANAGEMENT_KEY>' \
      -d '{"value":true}' \
      http://localhost:8317/v0/management/usage-statistics-enabled
    ```
  - 响应：
    ```json
    { "status": "ok" }
    ```

### 代理服务器 URL
- GET `/proxy-url` — 获取代理 URL 字符串
  - 请求：
    ```bash
    curl -H 'Authorization: Bearer <MANAGEMENT_KEY>' http://localhost:8317/v0/management/proxy-url
    ```
  - 响应：
    ```json
    { "proxy-url": "socks5://user:pass@127.0.0.1:1080/" }
    ```
- PUT/PATCH `/proxy-url` — 设置代理 URL 字符串
  - 请求（PUT）：
    ```bash
    curl -X PUT -H 'Content-Type: application/json' \
    -H 'Authorization: Bearer <MANAGEMENT_KEY>' \
      -d '{"value":"socks5://user:pass@127.0.0.1:1080/"}' \
      http://localhost:8317/v0/management/proxy-url
    ```
  - 请求（PATCH）：
    ```bash
    curl -X PATCH -H 'Content-Type: application/json' \
    -H 'Authorization: Bearer <MANAGEMENT_KEY>' \
      -d '{"value":"http://127.0.0.1:8080"}' \
      http://localhost:8317/v0/management/proxy-url
    ```
  - 响应：
    ```json
    { "status": "ok" }
    ```
- DELETE `/proxy-url` — 清空代理 URL
  - 请求：
    ```bash
    curl -H 'Authorization: Bearer <MANAGEMENT_KEY>' -X DELETE http://localhost:8317/v0/management/proxy-url
    ```
  - 响应：
    ```json
    { "status": "ok" }
    ```

### 超出配额行为
- GET `/quota-exceeded/switch-project`
  - 请求：
    ```bash
    curl -H 'Authorization: Bearer <MANAGEMENT_KEY>' http://localhost:8317/v0/management/quota-exceeded/switch-project
    ```
  - 响应：
    ```json
    { "switch-project": true }
    ```
- PUT/PATCH `/quota-exceeded/switch-project` — 布尔值
  - 请求：
    ```bash
    curl -X PUT -H 'Content-Type: application/json' \
    -H 'Authorization: Bearer <MANAGEMENT_KEY>' \
      -d '{"value":false}' \
      http://localhost:8317/v0/management/quota-exceeded/switch-project
    ```
  - 响应：
    ```json
    { "status": "ok" }
    ```
- GET `/quota-exceeded/switch-preview-model`
  - 请求：
    ```bash
    curl -H 'Authorization: Bearer <MANAGEMENT_KEY>' http://localhost:8317/v0/management/quota-exceeded/switch-preview-model
    ```
  - 响应：
    ```json
    { "switch-preview-model": true }
    ```
- PUT/PATCH `/quota-exceeded/switch-preview-model` — 布尔值
  - 请求：
    ```bash
    curl -X PATCH -H 'Content-Type: application/json' \
    -H 'Authorization: Bearer <MANAGEMENT_KEY>' \
      -d '{"value":true}' \
      http://localhost:8317/v0/management/quota-exceeded/switch-preview-model
    ```
  - 响应：
    ```json
    { "status": "ok" }
    ```

### API Keys（代理服务认证）
这些接口会更新配置中 `auth.providers` 内置的 `config-api-key` 提供方，旧版顶层 `api-keys` 会自动保持同步。
- GET `/api-keys` — 返回完整列表
  - 请求：
    ```bash
    curl -H 'Authorization: Bearer <MANAGEMENT_KEY>' http://localhost:8317/v0/management/api-keys
    ```
  - 响应：
    ```json
    { "api-keys": ["k1","k2","k3"] }
    ```
- PUT `/api-keys` — 完整改写列表
  - 请求：
    ```bash
    curl -X PUT -H 'Content-Type: application/json' \
    -H 'Authorization: Bearer <MANAGEMENT_KEY>' \
      -d '["k1","k2","k3"]' \
      http://localhost:8317/v0/management/api-keys
    ```
  - 响应：
    ```json
    { "status": "ok" }
    ```
- PATCH `/api-keys` — 修改其中一个（`old/new` 或 `index/value`）
  - 请求（按 old/new）：
    ```bash
    curl -X PATCH -H 'Content-Type: application/json' \
    -H 'Authorization: Bearer <MANAGEMENT_KEY>' \
      -d '{"old":"k2","new":"k2b"}' \
      http://localhost:8317/v0/management/api-keys
    ```
  - 请求（按 index/value）：
    ```bash
    curl -X PATCH -H 'Content-Type: application/json' \
    -H 'Authorization: Bearer <MANAGEMENT_KEY>' \
      -d '{"index":0,"value":"k1b"}' \
      http://localhost:8317/v0/management/api-keys
    ```
  - 响应：
    ```json
    { "status": "ok" }
    ```
- DELETE `/api-keys` — 删除其中一个（`?value=` 或 `?index=`）
  - 请求（按值删除）：
    ```bash
    curl -H 'Authorization: Bearer <MANAGEMENT_KEY>' -X DELETE 'http://localhost:8317/v0/management/api-keys?value=k1'
    ```
  - 请求（按索引删除）：
    ```bash
    curl -H 'Authorization: Bearer <MANAGEMENT_KEY>' -X DELETE 'http://localhost:8317/v0/management/api-keys?index=0'
    ```
  - 响应：
    ```json
    { "status": "ok" }
    ```

### Gemini API Key
- GET `/gemini-api-key`
  - 请求：
    ```bash
    curl -H 'Authorization: Bearer <MANAGEMENT_KEY>' http://localhost:8317/v0/management/gemini-api-key
    ```
  - 响应：
    ```json
    {
      "gemini-api-key": [
        {"api-key":"AIzaSy...01","base-url":"https://generativelanguage.googleapis.com","headers":{"X-Custom-Header":"custom-value"},"proxy-url":""},
        {"api-key":"AIzaSy...02","proxy-url":"socks5://proxy.example.com:1080"}
      ]
    }
    ```
- PUT `/gemini-api-key`
  - 请求（数组形式）：
    ```bash
    curl -X PUT -H 'Content-Type: application/json' \
    -H 'Authorization: Bearer <MANAGEMENT_KEY>' \
      -d '[{"api-key":"AIzaSy-1","headers":{"X-Custom-Header":"vendor-value"}},{"api-key":"AIzaSy-2","base-url":"https://custom.example.com"}]' \
      http://localhost:8317/v0/management/gemini-api-key
    ```
  - 响应：
    ```json
    { "status": "ok" }
    ```
- PATCH `/gemini-api-key`
  - 请求（按索引更新）：
    ```bash
    curl -X PATCH -H 'Content-Type: application/json' \
    -H 'Authorization: Bearer <MANAGEMENT_KEY>' \
      -d '{"index":0,"value":{"api-key":"AIzaSy-1","base-url":"https://custom.example.com","headers":{"X-Custom-Header":"custom-value"},"proxy-url":""}}' \
      http://localhost:8317/v0/management/gemini-api-key
    ```
  - 请求（按 api-key 匹配更新）：
    ```bash
    curl -X PATCH -H 'Content-Type: application/json' \
    -H 'Authorization: Bearer <MANAGEMENT_KEY>' \
      -d '{"match":"AIzaSy-1","value":{"api-key":"AIzaSy-1","headers":{"X-Custom-Header":"custom-value"},"proxy-url":"socks5://proxy.example.com:1080"}}' \
      http://localhost:8317/v0/management/gemini-api-key
    ```
  - 响应：
    ```json
    { "status": "ok" }
    ```
- DELETE `/gemini-api-key`
  - 请求（按 api-key 删除）：
    ```bash
    curl -H 'Authorization: Bearer <MANAGEMENT_KEY>' -X DELETE \
      'http://localhost:8317/v0/management/gemini-api-key?api-key=AIzaSy-1'
    ```
  - 请求（按索引删除）：
    ```bash
    curl -H 'Authorization: Bearer <MANAGEMENT_KEY>' -X DELETE \
      'http://localhost:8317/v0/management/gemini-api-key?index=0'
    ```
  - 响应：
    ```json
    { "status": "ok" }
    ```

### Generative Language API Key（兼容接口）
- GET `/generative-language-api-key`
  - 请求：
    ```bash
    curl -H 'Authorization: Bearer <MANAGEMENT_KEY>' http://localhost:8317/v0/management/generative-language-api-key
    ```
  - 响应：
    ```json
    { "generative-language-api-key": ["AIzaSy...01","AIzaSy...02"] }
    ```
- PUT `/generative-language-api-key`
  - 请求：
    ```bash
    curl -X PUT -H 'Content-Type: application/json' \
    -H 'Authorization: Bearer <MANAGEMENT_KEY>' \
      -d '["AIzaSy-1","AIzaSy-2"]' \
      http://localhost:8317/v0/management/generative-language-api-key
    ```
  - 响应：
    ```json
    { "status": "ok" }
    ```
- PATCH `/generative-language-api-key`
  - 请求：
    ```bash
    curl -X PATCH -H 'Content-Type: application/json' \
    -H 'Authorization: Bearer <MANAGEMENT_KEY>' \
      -d '{"old":"AIzaSy-1","new":"AIzaSy-1b"}' \
      http://localhost:8317/v0/management/generative-language-api-key
    ```
  - 响应：
    ```json
    { "status": "ok" }
    ```
- DELETE `/generative-language-api-key`
  - 请求：
    ```bash
    curl -H 'Authorization: Bearer <MANAGEMENT_KEY>' -X DELETE 'http://localhost:8317/v0/management/generative-language-api-key?value=AIzaSy-2'
    ```
  - 响应：
    ```json
    { "status": "ok" }
    ```
- 说明：
  - 该接口只读写纯字符串列表，实际上会映射到 `gemini-api-key`。

### Codex API KEY（对象数组）
- GET `/codex-api-key` — 列出全部
    - 请求：
      ```bash
      curl -H 'Authorization: Bearer <MANAGEMENT_KEY>' http://localhost:8317/v0/management/codex-api-key
      ```
    - 响应：
      ```json
      { "codex-api-key": [ { "api-key": "sk-a", "base-url": "", "proxy-url": "" } ] }
      ```
- PUT `/codex-api-key` — 完整改写列表
    - 请求：
      ```bash
      curl -X PUT -H 'Content-Type: application/json' \
      -H 'Authorization: Bearer <MANAGEMENT_KEY>' \
        -d '[{"api-key":"sk-a","proxy-url":"socks5://proxy.example.com:1080"},{"api-key":"sk-b","base-url":"https://c.example.com","proxy-url":""}]' \
        http://localhost:8317/v0/management/codex-api-key
      ```
    - 响应：
      ```json
      { "status": "ok" }
      ```
- PATCH `/codex-api-key` — 修改其中一个（按 `index` 或 `match`）
    - 请求（按索引）：
      ```bash
      curl -X PATCH -H 'Content-Type: application/json' \
      -H 'Authorization: Bearer <MANAGEMENT_KEY>' \
        -d '{"index":1,"value":{"api-key":"sk-b2","base-url":"https://c.example.com","proxy-url":""}}' \
        http://localhost:8317/v0/management/codex-api-key
      ```
    - 请求（按匹配）：
      ```bash
      curl -X PATCH -H 'Content-Type: application/json' \
      -H 'Authorization: Bearer <MANAGEMENT_KEY>' \
        -d '{"match":"sk-a","value":{"api-key":"sk-a","base-url":"","proxy-url":"socks5://proxy.example.com:1080"}}' \
        http://localhost:8317/v0/management/codex-api-key
      ```
    - 响应：
      ```json
      { "status": "ok" }
      ```
- DELETE `/codex-api-key` — 删除其中一个（`?api-key=` 或 `?index=`）
    - 请求（按 api-key）：
      ```bash
      curl -H 'Authorization: Bearer <MANAGEMENT_KEY>' -X DELETE 'http://localhost:8317/v0/management/codex-api-key?api-key=sk-b2'
      ```
    - 请求（按索引）：
      ```bash
      curl -H 'Authorization: Bearer <MANAGEMENT_KEY>' -X DELETE 'http://localhost:8317/v0/management/codex-api-key?index=0'
      ```
    - 响应：
      ```json
      { "status": "ok" }
      ```

### 请求重试次数
- GET `/request-retry` — 获取整数
  - 请求：
    ```bash
    curl -H 'Authorization: Bearer <MANAGEMENT_KEY>' http://localhost:8317/v0/management/request-retry
    ```
  - 响应：
    ```json
    { "request-retry": 3 }
    ```
- PUT/PATCH `/request-retry` — 设置整数
  - 请求：
    ```bash
    curl -X PATCH -H 'Content-Type: application/json' \
    -H 'Authorization: Bearer <MANAGEMENT_KEY>' \
      -d '{"value":5}' \
      http://localhost:8317/v0/management/request-retry
    ```
  - 响应：
    ```json
    { "status": "ok" }
    ```

### 请求日志开关
- GET `/request-log` — 获取布尔值
  - 请求：
    ```bash
    curl -H 'Authorization: Bearer <MANAGEMENT_KEY>' http://localhost:8317/v0/management/request-log
    ```
  - 响应：
    ```json
    { "request-log": false }
    ```
- PUT/PATCH `/request-log` — 设置布尔值
  - 请求：
    ```bash
    curl -X PATCH -H 'Content-Type: application/json' \
    -H 'Authorization: Bearer <MANAGEMENT_KEY>' \
      -d '{"value":true}' \
      http://localhost:8317/v0/management/request-log
    ```
  - 响应：
    ```json
    { "status": "ok" }
    ```

### Claude API KEY（对象数组）
- GET `/claude-api-key` — 列出全部
    - 请求：
      ```bash
      curl -H 'Authorization: Bearer <MANAGEMENT_KEY>' http://localhost:8317/v0/management/claude-api-key
      ```
    - 响应：
      ```json
      { "claude-api-key": [ { "api-key": "sk-a", "base-url": "", "proxy-url": "" } ] }
      ```
- PUT `/claude-api-key` — 完整改写列表
    - 请求：
      ```bash
      curl -X PUT -H 'Content-Type: application/json' \
      -H 'Authorization: Bearer <MANAGEMENT_KEY>' \
        -d '[{"api-key":"sk-a","proxy-url":"socks5://proxy.example.com:1080"},{"api-key":"sk-b","base-url":"https://c.example.com","proxy-url":""}]' \
        http://localhost:8317/v0/management/claude-api-key
      ```
  - 响应：
    ```json
    { "status": "ok" }
    ```
- PATCH `/claude-api-key` — 修改其中一个（按 `index` 或 `match`）
  - 请求（按索引）：
    ```bash
    curl -X PATCH -H 'Content-Type: application/json' \
    -H 'Authorization: Bearer <MANAGEMENT_KEY>' \
        -d '{"index":1,"value":{"api-key":"sk-b2","base-url":"https://c.example.com","proxy-url":""}}' \
        http://localhost:8317/v0/management/claude-api-key
      ```
  - 请求（按匹配）：
    ```bash
    curl -X PATCH -H 'Content-Type: application/json' \
    -H 'Authorization: Bearer <MANAGEMENT_KEY>' \
        -d '{"match":"sk-a","value":{"api-key":"sk-a","base-url":"","proxy-url":"socks5://proxy.example.com:1080"}}' \
        http://localhost:8317/v0/management/claude-api-key
      ```
  - 响应：
    ```json
    { "status": "ok" }
    ```
- DELETE `/claude-api-key` — 删除其中一个（`?api-key=` 或 `?index=`）
  - 请求（按 api-key）：
    ```bash
    curl -H 'Authorization: Bearer <MANAGEMENT_KEY>' -X DELETE 'http://localhost:8317/v0/management/claude-api-key?api-key=sk-b2'
    ```
  - 请求（按索引）：
    ```bash
    curl -H 'Authorization: Bearer <MANAGEMENT_KEY>' -X DELETE 'http://localhost:8317/v0/management/claude-api-key?index=0'
    ```
  - 响应：
    ```json
    { "status": "ok" }
    ```

### OpenAI 兼容提供商（对象数组）
- GET `/openai-compatibility` — 列出全部
  - 请求：
    ```bash
    curl -H 'Authorization: Bearer <MANAGEMENT_KEY>' http://localhost:8317/v0/management/openai-compatibility
    ```
  - 响应：
    ```json
    { "openai-compatibility": [ { "name": "openrouter", "base-url": "https://openrouter.ai/api/v1", "api-key-entries": [ { "api-key": "sk", "proxy-url": "" } ], "models": [] } ] }
    ```
- PUT `/openai-compatibility` — 完整改写列表
  - 请求：
    ```bash
    curl -X PUT -H 'Content-Type: application/json' \
    -H 'Authorization: Bearer <MANAGEMENT_KEY>' \
      -d '[{"name":"openrouter","base-url":"https://openrouter.ai/api/v1","api-key-entries":[{"api-key":"sk","proxy-url":""}],"models":[{"name":"m","alias":"a"}]}]' \
      http://localhost:8317/v0/management/openai-compatibility
    ```
  - 响应：
    ```json
    { "status": "ok" }
    ```
- PATCH `/openai-compatibility` — 修改其中一个（按 `index` 或 `name`）
  - 请求（按名称）：
    ```bash
    curl -X PATCH -H 'Content-Type: application/json' \
    -H 'Authorization: Bearer <MANAGEMENT_KEY>' \
      -d '{"name":"openrouter","value":{"name":"openrouter","base-url":"https://openrouter.ai/api/v1","api-key-entries":[{"api-key":"sk","proxy-url":""}],"models":[]}}' \
      http://localhost:8317/v0/management/openai-compatibility
    ```
  - 请求（按索引）：
    ```bash
    curl -X PATCH -H 'Content-Type: application/json' \
    -H 'Authorization: Bearer <MANAGEMENT_KEY>' \
      -d '{"index":0,"value":{"name":"openrouter","base-url":"https://openrouter.ai/api/v1","api-key-entries":[{"api-key":"sk","proxy-url":""}],"models":[]}}' \
      http://localhost:8317/v0/management/openai-compatibility
    ```
  - 响应：
    ```json
    { "status": "ok" }
    ```

  - 说明：
    - 仍可提交遗留的 `api-keys` 字段，但所有密钥会自动迁移到 `api-key-entries` 中，返回结果中的 `api-keys` 会逐步留空。
- DELETE `/openai-compatibility` — 删除（`?name=` 或 `?index=`）
  - 请求（按名称）：
    ```bash
    curl -H 'Authorization: Bearer <MANAGEMENT_KEY>' -X DELETE 'http://localhost:8317/v0/management/openai-compatibility?name=openrouter'
    ```
  - 请求（按索引）：
    ```bash
    curl -H 'Authorization: Bearer <MANAGEMENT_KEY>' -X DELETE 'http://localhost:8317/v0/management/openai-compatibility?index=0'
    ```
  - 响应：
    ```json
    { "status": "ok" }
    ```

### 认证文件管理

管理 `auth-dir` 下的 JSON 令牌文件：列出、下载、上传、删除。

- GET `/auth-files` — 列表
  - 请求：
    ```bash
    curl -H 'Authorization: Bearer <MANAGEMENT_KEY>' http://localhost:8317/v0/management/auth-files
    ```
  - 响应：
    ```json
    { "files": [ { "name": "acc1.json", "size": 1234, "modtime": "2025-08-30T12:34:56Z", "type": "google", "email": "user@example.com" } ] }
    ```
  - 说明：
    - `modtime` 使用 RFC3339 格式；若文件中包含邮箱字段则一并返回。

- GET `/auth-files/download?name=<file.json>` — 下载单个文件
  - 请求：
    ```bash
    curl -H 'Authorization: Bearer <MANAGEMENT_KEY>' -OJ 'http://localhost:8317/v0/management/auth-files/download?name=acc1.json'
    ```

- POST `/auth-files` — 上传
  - 请求（multipart）：
    ```bash
    curl -X POST -F 'file=@/path/to/acc1.json' \
    -H 'Authorization: Bearer <MANAGEMENT_KEY>' \
      http://localhost:8317/v0/management/auth-files
    ```
  - 请求（原始 JSON）：
    ```bash
    curl -X POST -H 'Content-Type: application/json' \
    -H 'Authorization: Bearer <MANAGEMENT_KEY>' \
      -d @/path/to/acc1.json \
      'http://localhost:8317/v0/management/auth-files?name=acc1.json'
    ```
  - 响应：
    ```json
    { "status": "ok" }
    ```
  - 说明：
    - 需确保核心认证管理器已启用，否则会以 `503` 返回 `{ "error": "core auth manager unavailable" }`。
    - 上传的文件名必须以 `.json` 结尾。

- DELETE `/auth-files?name=<file.json>` — 删除单个文件
  - 请求：
    ```bash
    curl -H 'Authorization: Bearer <MANAGEMENT_KEY>' -X DELETE 'http://localhost:8317/v0/management/auth-files?name=acc1.json'
    ```
  - 响应：
    ```json
    { "status": "ok" }
    ```

- DELETE `/auth-files?all=true` — 删除 `auth-dir` 下所有 `.json` 文件
  - 请求：
    ```bash
    curl -H 'Authorization: Bearer <MANAGEMENT_KEY>' -X DELETE 'http://localhost:8317/v0/management/auth-files?all=true'
    ```
  - 响应：
    ```json
    { "status": "ok", "deleted": 3 }
    ```

### 登录/授权 URL

以下端点用于发起各提供商的登录流程，并返回需要在浏览器中打开的 URL。流程完成后，令牌会保存到 `auths/` 目录。

对于 Anthropic、Codex、Gemini CLI 与 iFlow，可附加 `?is_webui=true` 以便从管理界面复用内置回调转发。

- GET `/anthropic-auth-url` — 开始 Anthropic（Claude）登录
  - 请求：
    ```bash
    curl -H 'Authorization: Bearer <MANAGEMENT_KEY>' \
      http://localhost:8317/v0/management/anthropic-auth-url
    ```
  - 响应：
    ```json
    { "status": "ok", "url": "https://...", "state": "anth-1716206400" }
    ```
  - 说明：
    - 若从 Web UI 触发，可添加 `?is_webui=true` 复用本地回调服务。

- GET `/codex-auth-url` — 开始 Codex 登录
  - 请求：
    ```bash
    curl -H 'Authorization: Bearer <MANAGEMENT_KEY>' \
      http://localhost:8317/v0/management/codex-auth-url
    ```
  - 响应：
    ```json
    { "status": "ok", "url": "https://...", "state": "codex-1716206400" }
    ```

- GET `/gemini-cli-auth-url` — 开始 Google（Gemini CLI）登录
  - 查询参数：
    - `project_id`（可选）：Google Cloud 项目 ID。
  - 请求：
    ```bash
    curl -H 'Authorization: Bearer <MANAGEMENT_KEY>' \
      'http://localhost:8317/v0/management/gemini-cli-auth-url?project_id=<PROJECT_ID>'
    ```
  - 响应：
    ```json
    { "status": "ok", "url": "https://...", "state": "gem-1716206400" }
    ```

- GET `/qwen-auth-url` — 开始 Qwen 登录（设备授权流程）
  - 请求：
    ```bash
    curl -H 'Authorization: Bearer <MANAGEMENT_KEY>' \
      http://localhost:8317/v0/management/qwen-auth-url
    ```
  - 响应：
    ```json
    { "status": "ok", "url": "https://...", "state": "gem-1716206400" }
    ```

- GET `/iflow-auth-url` — 开始 iFlow 登录
  - 请求：
    ```bash
    curl -H 'Authorization: Bearer <MANAGEMENT_KEY>' \
      http://localhost:8317/v0/management/iflow-auth-url
    ```
  - 响应：
    ```json
    { "status": "ok", "url": "https://...", "state": "ifl-1716206400" }
    ```

- GET `/get-auth-status?state=<state>` — 轮询 OAuth 流程状态
  - 请求：
    ```bash
    curl -H 'Authorization: Bearer <MANAGEMENT_KEY>' \
      'http://localhost:8317/v0/management/get-auth-status?state=<STATE_FROM_AUTH_URL>'
    ```
  - 响应示例：
    ```json
    { "status": "wait" }
    { "status": "ok" }
    { "status": "error", "error": "Authentication failed" }
    ```

## 错误响应

通用错误格式：
- 400 Bad Request: `{ "error": "invalid body" }`
- 401 Unauthorized: `{ "error": "missing management key" }` 或 `{ "error": "invalid management key" }`
- 403 Forbidden: `{ "error": "remote management disabled" }`
- 404 Not Found: `{ "error": "item not found" }` 或 `{ "error": "file not found" }`
- 422 Unprocessable Entity: `{ "error": "invalid_config", "message": "..." }`
- 500 Internal Server Error: `{ "error": "failed to save config: ..." }`
- 503 Service Unavailable: `{ "error": "core auth manager unavailable" }`

## 说明

- 变更会写回 YAML 配置文件，并由文件监控器热重载配置与客户端。
- `allow-remote-management` 与 `remote-management-key` 不能通过 API 修改，需在配置文件中设置。
