package management

// NOTE: This file has been decomposed into smaller, logically-grouped files:
// - auth_helpers.go: Callback forwarders, timestamp parsing, auth helpers
// - auth_file_mgmt.go: Auth file listing, building entries, metadata extraction
// - auth_file_crud.go: Download, upload, delete auth files
// - auth_file_patch.go: Patch status/fields, token record management
// - auth_anthropic.go: Anthropic token requests
// - auth_gemini.go: Gemini CLI token, setup, helpers
// - auth_codex.go: Codex token requests
// - auth_antigravity.go: Antigravity token requests
// - auth_qwen.go: Qwen token requests
// - auth_kimi.go: Kimi token requests
// - auth_iflow.go: IFlow and IFlow Cookie token requests
// - auth_github.go: GitHub token requests
// - auth_kiro.go: Kiro token requests and PKCE helpers
// - auth_kilo.go: Kilo token requests
// - auth_status.go: Auth status endpoint
//
// All functions remain in the 'management' package. This is purely a file organization change.
// No function signatures or behavior have been modified.
