package translator

import (
	"context"

	. "github.com/kooshapari/CLIProxyAPI/v7/pkg/llmproxy/constant"
	"github.com/kooshapari/CLIProxyAPI/v7/pkg/llmproxy/interfaces"
)

// init registers built-in pass-through translators that do not belong to a
// dedicated translator sub-package. The OpenAI -> OpenAI mapping is an identity
// transform: requests and responses are forwarded unchanged. Registering it here
// (rather than relying on an external sub-package import) guarantees the
// pass-through is always available to consumers of this package, including
// callers that only depend on the translator registry itself.
func init() {
	Register(
		OpenAI,
		OpenAI,
		func(_ string, rawJSON []byte, _ bool) []byte {
			return rawJSON
		},
		interfaces.TranslateResponse{
			Stream: func(_ context.Context, _ string, _, _, rawJSON []byte, _ *any) [][]byte {
				return [][]byte{rawJSON}
			},
			NonStream: func(_ context.Context, _ string, _, _, rawJSON []byte, _ *any) []byte {
				return rawJSON
			},
		},
	)
}
