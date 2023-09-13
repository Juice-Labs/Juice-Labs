/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package logger

import (
	"flag"
	"fmt"
	"strings"

	"go.uber.org/zap"
)

var (
	quiet       = flag.Bool("quiet", false, "Disables all logging output")
	logLevelArg = flag.String("log-level", "info", "Sets the maximum level of output [Fatal, Error, Warning, Info (Default), Debug]")
	logFile     = flag.String("log-file", "", "")
	logFormat   = flag.String("log-format", "juice", "Set the format of the logging [juice, console, json]")

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

	zap.RegisterEncoder("juice", NewJuiceEncoder)

	config := zap.NewDevelopmentConfig()
	config.Encoding = *logFormat
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
			return fmt.Errorf("failed to initialize logger, %w", err)
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
	sugardLogger.Error(v...)
}

func Errorf(format string, v ...any) {
	sugardLogger.Errorf(format, v...)
}

func Warning(v ...any) {
	sugardLogger.Warn(v...)
}

func Warningf(format string, v ...any) {
	sugardLogger.Warnf(format, v...)
}

func Info(v ...any) {
	sugardLogger.Info(v...)
}

func Infof(format string, v ...any) {
	sugardLogger.Infof(format, v...)
}

func Debug(v ...any) {
	sugardLogger.Debug(v...)
}

func Debugf(format string, v ...any) {
	sugardLogger.Debugf(format, v...)
}
