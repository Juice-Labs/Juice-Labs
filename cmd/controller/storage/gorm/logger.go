package gorm

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Xdevlab/Run/pkg/logger"
	glogger "gorm.io/gorm/logger"
	"gorm.io/gorm/utils"
)

// Copy-pasta logger from GORM which feeds the logs into our logger
type gormLogger struct {
	LogLevel                  glogger.LogLevel
	SlowThreshold             time.Duration
	IgnoreRecordNotFoundError bool
}

func NewLogger(config glogger.Config) gormLogger {
	return gormLogger{
		LogLevel:                  config.LogLevel,
		SlowThreshold:             config.SlowThreshold,
		IgnoreRecordNotFoundError: config.IgnoreRecordNotFoundError,
	}
}

func (l gormLogger) LogMode(level glogger.LogLevel) glogger.Interface {
	// Ignore, use logger log level
	return l
}

func (l gormLogger) Info(ctx context.Context, message string, data ...interface{}) {
	if l.LogLevel >= glogger.Info {
		logger.Infof(message, data...)
	}
}

func (l gormLogger) Warn(ctx context.Context, message string, data ...interface{}) {
	if l.LogLevel >= glogger.Warn {
		logger.Warningf(message, data...)
	}
}

func (l gormLogger) Error(ctx context.Context, message string, data ...interface{}) {
	if l.LogLevel >= glogger.Error {
		logger.Errorf(message, data...)
	}
}

func (l gormLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	if l.LogLevel <= glogger.Silent {
		return
	}

	elapsed := time.Since(begin)
	switch {
	case err != nil && (!errors.Is(err, glogger.ErrRecordNotFound) || !l.IgnoreRecordNotFoundError):
		sql, rows := fc()
		if rows == -1 {
			logger.Error(err, "-", " ", sql)
		} else {
			logger.Error(err, rows, " ", sql)
		}
	case elapsed > l.SlowThreshold && l.SlowThreshold != 0 && l.LogLevel >= glogger.Warn:
		sql, rows := fc()
		slowLog := fmt.Sprintf("SLOW SQL >= %v", l.SlowThreshold)
		if rows == -1 {
			logger.Warningf("%s %s %f %s", utils.FileWithLineNum(), slowLog, float64(elapsed.Nanoseconds())/1e6, sql)
		} else {
			logger.Warning("%s %s %f %d %s", utils.FileWithLineNum(), slowLog, float64(elapsed.Nanoseconds())/1e6, rows, sql)
		}
	case l.LogLevel == glogger.Info:
		sql, rows := fc()
		if rows == -1 {
			logger.Infof("%s %f %s", utils.FileWithLineNum(), float64(elapsed.Nanoseconds())/1e6, sql)
		} else {
			logger.Infof("%s %f %d %s", utils.FileWithLineNum(), float64(elapsed.Nanoseconds())/1e6, rows, sql)
		}
	}
}
