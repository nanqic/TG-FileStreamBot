package utils

import (
	"EverythingSuckz/fsb/config"
	"os"
	"sync"

	"github.com/gotd/td/tgerr"
	"go.uber.org/zap"
)

// Telegram errors that mean the client's connection/session is broken and
// won't recover by simply retrying the same request on the same client.
var tgConnectionErrorTypes = []string{
	"CONNECTION_NOT_INITED",
	"CONNECTION_LAYER_INVALID",
	"AUTH_KEY_UNREGISTERED",
	"AUTH_KEY_INVALID",
	"AUTH_KEY_PERM_EMPTY",
	"SESSION_REVOKED",
}

// IsTGConnectionError reports whether err is a broken-connection error from Telegram.
func IsTGConnectionError(err error) bool {
	return err != nil && tgerr.Is(err, tgConnectionErrorTypes...)
}

var (
	tgErrMu     sync.Mutex
	tgErrCounts = map[int64]int{}
)

// ReportTGResult records the outcome of a real Telegram RPC made by clientID.
// A success resets the client's counter; a connection error increments it.
// Once a client hits TG_ERROR_EXIT_THRESHOLD consecutive connection errors the
// process exits with code 1, so systemd (Restart=always) / docker restarts it
// with fresh connections. Other errors (timeouts, cancellations...) are ignored.
func ReportTGResult(clientID int64, err error) {
	if err == nil {
		tgErrMu.Lock()
		delete(tgErrCounts, clientID)
		tgErrMu.Unlock()
		return
	}
	if !IsTGConnectionError(err) {
		return
	}
	tgErrMu.Lock()
	tgErrCounts[clientID]++
	n := tgErrCounts[clientID]
	tgErrMu.Unlock()

	log := Logger.Named("TGHealth")
	threshold := config.ValueOf.TGErrorExitThreshold
	log.Warn("Telegram connection error",
		zap.Int64("clientID", clientID),
		zap.Int("consecutive", n),
		zap.Int("threshold", threshold),
		zap.Error(err),
	)
	if threshold > 0 && n >= threshold {
		log.Error("Too many consecutive Telegram connection errors, exiting so the supervisor can restart the process",
			zap.Int64("clientID", clientID))
		_ = Logger.Sync()
		os.Exit(1)
	}
}
