// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package devices

/*
type ctxKey struct{}

// ContextWithLogger returns a new context with the given logger.
func ContextWithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey(struct{}{}), logger)
}

var discardLogger = slog.New(slog.NewJSONHandler(io.Discard, nil))

// LoggerFromContext returns the logger from the given context.
// If no logger is set, it returns a discard logger.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	l := ctx.Value(ctxKey(struct{}{}))
	if l == nil {
		return discardLogger
	}
	return l.(*slog.Logger)
}

// ContextWithLoggerAttributes returns a new context with the embedded logger
// updated with the given logger attributes.
func ContextWithLoggerAttributes(ctx context.Context, attributes ...any) context.Context {
	l := ctx.Value(ctxKey(struct{}{}))
	if l == nil {
		return ctx
	}
	return ContextWithLogger(ctx, l.(*slog.Logger).With(attributes...))
}
*/
