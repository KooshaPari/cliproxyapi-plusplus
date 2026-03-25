package synthesizer

import (
	"time"

<<<<<<< HEAD
	"github.com/kooshapari/cliproxyapi-plusplus/v6/internal/config"
=======
	"github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/config"
>>>>>>> origin/main
)

// SynthesisContext provides the context needed for auth synthesis.
type SynthesisContext struct {
	// Config is the current configuration
	Config *config.Config
	// AuthDir is the directory containing auth files
	AuthDir string
	// Now is the current time for timestamps
	Now time.Time
	// IDGenerator generates stable IDs for auth entries
	IDGenerator *StableIDGenerator
}
