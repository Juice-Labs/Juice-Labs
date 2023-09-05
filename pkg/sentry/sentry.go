package sentry

import (
	"fmt"
	"os"
	"time"

	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	SentryDsn = ""
)

type ClientOptions = sentry.ClientOptions

func Initialize(config sentry.ClientOptions) error {
	var err error

	// Use config DSN if available, falling back to SENTRY_DSN, or package level (build time) DSN
	if config.Dsn == "" {
		config.Dsn = os.Getenv("SENTRY_DSN")
		if config.Dsn == "" {
			config.Dsn = SentryDsn
		}
	}

	if config.Dsn != "" {
		err = sentry.Init(config)

		if err == nil {
			// add a logger hook so sentry gets notified of errors etc
			logger.AddOption(zap.Hooks(func(entry zapcore.Entry) error {
				if entry.Level == zapcore.ErrorLevel {
					sentry.AddBreadcrumb(&sentry.Breadcrumb{
						Type:      "error",
						Category:  "error",
						Level:     sentry.LevelError,
						Message:   fmt.Sprintf("%s %s", entry.Caller.TrimmedPath(), entry.Message),
						Timestamp: entry.Time,
					})
				}
				return nil
			}))
		}
	}

	return err

}

func Enabled() bool {
	return sentry.CurrentHub().Client() != nil
}

func Close() {
	if err := recover(); err != nil {
		sentry.CurrentHub().Recover(err)
		sentry.Flush(2 * time.Second)
		// re-raise panic
		panic(err)
	}
	sentry.Flush(2 * time.Second)
}
