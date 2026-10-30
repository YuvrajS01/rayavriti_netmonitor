package logging

import (
	"context"
	"log/slog"
)

// persistingHandler backs the process-wide slog default. It writes through
// the configured handler (level, format, file sink) and, like Logger, hands
// events to the runtime persist function. Package-level slog calls are
// spread across hot paths (scheduler, collectors, engine), so only Warn and
// above, or events matching a verbose session, are persisted.
type persistingHandler struct {
	inner  slog.Handler
	root   *Logger
	attrs  []slog.Attr
	prefix string
}

func newPersistingHandler(inner slog.Handler, root *Logger) *persistingHandler {
	return &persistingHandler{inner: inner, root: root}
}

func (h *persistingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *persistingHandler) Handle(ctx context.Context, r slog.Record) error {
	err := h.inner.Handle(ctx, r)

	attrs := make(map[string]any, len(h.attrs)+r.NumAttrs())
	for _, a := range h.attrs {
		attrs[h.prefix+a.Key] = a.Value.Any()
	}
	r.Attrs(func(a slog.Attr) bool {
		attrs[h.prefix+a.Key] = a.Value.Any()
		return true
	})
	if reqID := GetRequestID(ctx); reqID != "" {
		attrs["request_id"] = reqID
	}
	component, _ := attrs["component"].(string)

	var verboseID *int64
	if r.Level < slog.LevelWarn {
		verboseID = h.root.runtime.matchVerbose(r.Level, component, attrs)
		if verboseID == nil {
			return err
		}
	}
	evt := h.root.persistedEvent(r.Level, r.Message, attrs, verboseID)
	if evt.Component == "" {
		evt.Component = "app"
	}
	h.root.runtime.persistEvent(ctx, evt)
	return err
}

func (h *persistingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := *h
	next.inner = h.inner.WithAttrs(attrs)
	next.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &next
}

func (h *persistingHandler) WithGroup(name string) slog.Handler {
	next := *h
	next.inner = h.inner.WithGroup(name)
	next.prefix = h.prefix + name + "."
	return &next
}
