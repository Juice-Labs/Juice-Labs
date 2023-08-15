/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package logger

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

type LogLevel int

const (
	LevelFatal LogLevel = iota
	LevelError
	LevelWarning
	LevelInfo
	LevelDebug
	LevelTrace
)

var (
	quiet                = flag.Bool("quiet", false, "Disables all logging output")
	logLevelArg          = flag.String("log-level", "info", "Sets the maximum level of output [Fatal, Error, Warning, Info (Default), Debug, Trace]")
	logFilePath          = flag.String("log-file", "", "")
	logFile     *os.File = nil

	logLevel = LevelInfo

	panicLogger   = log.New(os.Stderr, "Panic: ", log.LstdFlags|log.LUTC|log.Lmsgprefix)
	fatalLogger   = log.New(os.Stderr, "Fatal: ", log.LstdFlags|log.LUTC|log.Lmsgprefix)
	errorLogger   = log.New(os.Stderr, "Error: ", log.LstdFlags|log.LUTC|log.Lmsgprefix)
	warningLogger = log.New(os.Stderr, "Warning: ", log.LstdFlags|log.LUTC|log.Lmsgprefix)
	infoLogger    = log.New(os.Stdout, "Info: ", log.LstdFlags|log.LUTC|log.Lmsgprefix)
	debugLogger   = log.New(os.Stdout, "Debug: ", log.LstdFlags|log.LUTC|log.Lmsgprefix)
	traceLogger   = log.New(os.Stdout, "Trace: ", log.LstdFlags|log.LUTC|log.Lmsgprefix)
)

func LogLevelAsString() (string, error) {
	switch logLevel {
	case LevelFatal:
		return "Fatal", nil
	case LevelError:
		return "Error", nil
	case LevelWarning:
		return "Warning", nil
	case LevelInfo:
		return "Info", nil
	case LevelDebug:
		return "Debug", nil
	case LevelTrace:
		return "Trace", nil
	}

	return "", fmt.Errorf("unknown log-level %d", logLevel)
}

func LogFile() *os.File {
	return logFile
}

func Configure() error {
	switch strings.ToLower(strings.TrimSpace(*logLevelArg)) {
	case "fatal":
		logLevel = LevelFatal
	case "error":
		logLevel = LevelError
	case "warning":
		logLevel = LevelWarning
	case "info":
		logLevel = LevelInfo
	case "debug":
		logLevel = LevelDebug
	case "trace":
		logLevel = LevelTrace
	default:
		return fmt.Errorf("unknown log-level %s", *logLevelArg)
	}

	var err error
	stdout := os.Stdout
	stderr := os.Stderr
	logFile = stdout

	if *logFilePath != "" {
		logFile, err = os.OpenFile(*logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return err
		}

		stdout = logFile
		stderr = logFile
	}

	var out io.Writer

	panicLogger = log.New(stderr, "Panic: ", log.LstdFlags|log.LUTC|log.Lmsgprefix)
	fatalLogger = log.New(stderr, "Fatal: ", log.LstdFlags|log.LUTC|log.Lmsgprefix)

	out = stderr
	if logLevel < LevelError {
		out = io.Discard
	}
	errorLogger = log.New(out, "Error: ", log.LstdFlags|log.LUTC|log.Lmsgprefix)

	out = stderr
	if logLevel < LevelWarning {
		out = io.Discard
	}
	warningLogger = log.New(out, "Warning: ", log.LstdFlags|log.LUTC|log.Lmsgprefix)

	out = stdout
	if logLevel < LevelInfo {
		out = io.Discard
	}
	infoLogger = log.New(out, "Info: ", log.LstdFlags|log.LUTC|log.Lmsgprefix)

	out = stdout
	if logLevel < LevelDebug {
		out = io.Discard
	}
	debugLogger = log.New(out, "Debug: ", log.LstdFlags|log.LUTC|log.Lmsgprefix)

	out = stdout
	if logLevel < LevelTrace {
		out = io.Discard
	}
	traceLogger = log.New(out, "Trace: ", log.LstdFlags|log.LUTC|log.Lmsgprefix)

	return nil
}

func Fatal(v ...any) {
	fatalLogger.Fatal(v...)
}

func Fatalf(format string, v ...any) {
	fatalLogger.Fatalf(format, v...)
}

func Panic(v ...any) {
	panicLogger.Panic(v...)
}

func Panicf(format string, v ...any) {
	panicLogger.Panicf(format, v...)
}

func Error(v ...any) {
	if !*quiet {
		errorLogger.Print(v...)
	}
}

func Errorf(format string, v ...any) {
	if !*quiet {
		errorLogger.Printf(format, v...)
	}
}

func Warning(v ...any) {
	if !*quiet {
		warningLogger.Print(v...)
	}
}

func Warningf(format string, v ...any) {
	if !*quiet {
		warningLogger.Printf(format, v...)
	}
}

func Info(v ...any) {
	if !*quiet {
		infoLogger.Print(v...)
	}
}

func Infof(format string, v ...any) {
	if !*quiet {
		infoLogger.Printf(format, v...)
	}
}

func Debug(v ...any) {
	if !*quiet {
		debugLogger.Print(v...)
	}
}

func Debugf(format string, v ...any) {
	if !*quiet {
		debugLogger.Printf(format, v...)
	}
}

func Trace(v ...any) {
	if !*quiet {
		traceLogger.Print(v...)
	}
}

func Tracef(format string, v ...any) {
	if !*quiet {
		traceLogger.Printf(format, v...)
	}
}
