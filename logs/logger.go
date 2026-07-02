package logs

import (
	"context"
	"log/slog"
)

type logIDHandler struct {
	slog.Handler
}

func (h *logIDHandler) Handle(ctx context.Context, r slog.Record) error {
	if logID := GetLogIDFromContext(ctx); logID != "" {
		r.AddAttrs(slog.String("log_id", logID))
	}
	return h.Handler.Handle(ctx, r)
}
