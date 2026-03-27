package minimax

import (
	"github.com/kooshapari/CLIProxyAPI/v7/pkg/llmproxy/thinking"
	"github.com/kooshapari/CLIProxyAPI/v7/pkg/llmproxy/thinking/provider/iflow"
)

// NewApplier returns a Minimax applier.
//
// Current Minimax implementations share the same request shape and behavior as iFlow
// models that use the reasoning_split toggle.
func NewApplier() *iflow.Applier {
	return iflow.NewApplier()
}

func init() {
	thinking.RegisterProvider("minimax", NewApplier())
}
