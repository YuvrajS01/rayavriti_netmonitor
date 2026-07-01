package logging

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
)

func attrsMap(args ...any) map[string]any {
	out := map[string]any{}
	for i := 0; i < len(args)-1; i += 2 {
		key, ok := args[i].(string)
		if !ok || key == "" {
			continue
		}
		out[key] = args[i+1]
	}
	return out
}

func redactAttrs(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		lk := strings.ToLower(k)
		if strings.Contains(lk, "password") ||
			strings.Contains(lk, "token") ||
			strings.Contains(lk, "secret") ||
			strings.Contains(lk, "authorization") ||
			strings.Contains(lk, "cookie") ||
			strings.Contains(lk, "community") ||
			strings.Contains(lk, "api_key") {
			out[k] = "***"
			continue
		}
		out[k] = v
	}
	return out
}

func levelName(level slog.Level) string {
	switch {
	case level <= LevelTrace:
		return "trace"
	case level < slog.LevelInfo:
		return "debug"
	case level < slog.LevelWarn:
		return "info"
	case level < slog.LevelError:
		return "warn"
	case level < LevelFatal:
		return "error"
	default:
		return "fatal"
	}
}

func attrString(attrs map[string]any, key string) string {
	v, ok := attrs[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	case int64:
		return strconv.FormatInt(t, 10)
	case int:
		return strconv.Itoa(t)
	case float64:
		return strconv.FormatInt(int64(t), 10)
	default:
		return fmt.Sprint(t)
	}
}

func firstString(attrs map[string]any, keys ...string) string {
	for _, key := range keys {
		if s := attrString(attrs, key); s != "" {
			return s
		}
	}
	return ""
}

func attrInt(attrs map[string]any, keys ...string) *int {
	for _, key := range keys {
		if v, ok := attrs[key]; ok {
			switch t := v.(type) {
			case int:
				return &t
			case int64:
				n := int(t)
				return &n
			case float64:
				n := int(t)
				return &n
			case string:
				if n, err := strconv.Atoi(t); err == nil {
					return &n
				}
			}
		}
	}
	return nil
}

func attrFloat(attrs map[string]any, key string) *float64 {
	v, ok := attrs[key]
	if !ok {
		return nil
	}
	switch t := v.(type) {
	case float64:
		return &t
	case float32:
		n := float64(t)
		return &n
	case int:
		n := float64(t)
		return &n
	case int64:
		n := float64(t)
		return &n
	case string:
		if n, err := strconv.ParseFloat(t, 64); err == nil {
			return &n
		}
	}
	return nil
}

func attrInt64(attrs map[string]any, key string) int64 {
	if v := attrOptionalInt64(attrs, key); v != nil {
		return *v
	}
	return 0
}

func attrOptionalInt64(attrs map[string]any, key string) *int64 {
	v, ok := attrs[key]
	if !ok {
		return nil
	}
	switch t := v.(type) {
	case int64:
		return &t
	case int:
		n := int64(t)
		return &n
	case float64:
		n := int64(t)
		return &n
	case string:
		if n, err := strconv.ParseInt(t, 10, 64); err == nil {
			return &n
		}
	}
	return nil
}
