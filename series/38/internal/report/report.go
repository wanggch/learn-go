package report

import (
	"fmt"
	"time"

	"learn-go/series/38/internal/config"
)

type Snapshot struct {
	Config  config.Config
	Handled int
	Failed  int
	Elapsed time.Duration
}

func Summary(s Snapshot) string {
	return fmt.Sprintf(
		"app=%s mode=%s region=%s handled=%d failed=%d cost=%s",
		s.Config.App,
		s.Config.Mode,
		s.Config.Region,
		s.Handled,
		s.Failed,
		s.Elapsed,
	)
}
