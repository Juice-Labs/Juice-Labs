/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package logger

import (
	"flag"
	"strings"

	"go.uber.org/zap"
)

var (
	quiet       = flag.Bool("quiet", false, "Disables all logging output")
	logLevelArg = flag.String("log-level", "info", "Sets the maximum level of output [Fatal, Error, Warning, Info (Default), Debug, Trace]")
	logFile     = flag.String("log-file", "", "")

	logLevel zap.AtomicLevel

	logger       *zap.Logger        = nil
	sugardLogger *zap.SugaredLogger = nil
	options      []zap.Option
)

func LogLevelAsString() (string, error) {
	return logLevel.String(), nil
}

func AddOption(option zap.Option) {
	options = append(options, option)
}

func Configure() error {
	var err error

	logLevel, err = zap.ParseAtomicLevel(strings.ToLower(strings.TrimSpace(*logLevelArg)))
	if err != nil {
		return err
	}

	config := zap.NewProductionConfig()
	config.Level = logLevel
	if *logFile != "" {
		config.OutputPaths = []string{
			*logFile,
		}
	}
	// Skip our logger api
	AddOption(zap.AddCallerSkip(1))

	if *quiet {
		logger = zap.NewNop()
	} else {
		logger, err = config.Build(options...)
		if err != nil {
			return err
		}
	}

	sugardLogger = logger.Sugar()
	return nil
}

func Close() {
	logger.Sync()
}

func Fatal(v ...any) {
	sugardLogger.Panic(v...)
}

func Fatalf(format string, v ...any) {
	sugardLogger.Panicf(format, v...)
}

func Panic(v ...any) {
	sugardLogger.Panic(v...)
}

func Panicf(format string, v ...any) {
	sugardLogger.Panicf(format, v...)
}

func Error(v ...any) {
	if !*quiet {
		sugardLogger.Error(v...)
	}
}

func Errorf(format string, v ...any) {
	if !*quiet {
		sugardLogger.Errorf(format, v...)
	}
}

func Warning(v ...any) {
	if !*quiet {
		sugardLogger.Warn(v...)
	}
}

func Warningf(format string, v ...any) {
	if !*quiet {
		sugardLogger.Warnf(format, v...)
	}
}

func Info(v ...any) {
	if !*quiet {
		sugardLogger.Info(v...)
	}
}

func Infof(format string, v ...any) {
	if !*quiet {
		sugardLogger.Infof(format, v...)
	}
}

func Debug(v ...any) {
	if !*quiet {
		sugardLogger.Debug(v...)
	}
}

func Debugf(format string, v ...any) {
	if !*quiet {
		sugardLogger.Debugf(format, v...)
	}
}

func Trace(v ...any) {
	if !*quiet {
		sugardLogger.Debug(v...)
	}
}

func Tracef(format string, v ...any) {
	if !*quiet {
		sugardLogger.Debugf(format, v...)
	}
}
