// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package devices

import (
	"context"
	"io"
	"log/slog"
)

type ctxKey string

// ContextWithLogger returns a new context with the given logger.
func ContextWithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey("logger"), logger)
}

var discardLogger = slog.New(slog.NewJSONHandler(io.Discard, nil))

// LoggerFromContext returns the logger from the given context.
// If no logger is set, it returns a discard logger.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	l := ctx.Value(ctxKey("logger"))
	if l == nil {
		return discardLogger
	}
	return l.(*slog.Logger)
}
