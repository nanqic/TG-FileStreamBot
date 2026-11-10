package utils

import (
	"time"

	"go.uber.org/zap"
)

// TimeFuncWithResult wraps a function call and logs its execution time, returning a result
func TimeFuncWithResult[T any](log *zap.Logger, funcName string, fn func() (T, error)) (T, error) {
	start := time.Now()
	defer func() {
		duration := time.Since(start).Milliseconds()
		log.Debug(funcName+" executed", zap.Int64("duration_ms", duration))
	}()
	return fn()
}

// TimeFuncWithResult2 is TimeFuncWithResult for functions returning two values.
func TimeFuncWithResult2[T, U any](log *zap.Logger, funcName string, fn func() (T, U, error)) (T, U, error) {
	start := time.Now()
	defer func() {
		duration := time.Since(start).Milliseconds()
		log.Debug(funcName+" executed", zap.Int64("duration_ms", duration))
	}()
	return fn()
}
