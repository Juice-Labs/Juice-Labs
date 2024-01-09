/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"os"

	"github.com/Xdevlab/Run/cmd/agent/app"
	"github.com/Xdevlab/Run/cmd/agent/playnite"
	"github.com/Xdevlab/Run/cmd/agent/prometheus"
	"github.com/Xdevlab/Run/cmd/internal/build"
	"github.com/Xdevlab/Run/pkg/appmain"
	"github.com/Xdevlab/Run/pkg/crypto"
	"github.com/Xdevlab/Run/pkg/logger"
	"github.com/Xdevlab/Run/pkg/sentry"
	"github.com/Xdevlab/Run/pkg/task"
	"github.com/joho/godotenv"
)

var (
	certFile     = flag.String("cert-file", "", "")
	keyFile      = flag.String("key-file", "", "")
	generateCert = flag.Bool("generate-cert", false, "Generates a certificate for https")
)

func main() {
	name := "Agent"
	config := appmain.Config{
		Name:    name,
		Version: build.Version,

		SentryConfig: sentry.ClientOptions{
			Dsn:              os.Getenv("JUICE_AGENT_SENTRY_DSN"),
			Release:          fmt.Sprintf("%s@%s", name, build.Version),
			EnableTracing:    true,
			TracesSampleRate: 1.0,
		},
	}

	err := appmain.Run(config, func(group task.Group) error {
		var tlsConfig *tls.Config

		if *certFile != "" && *keyFile != "" {
			certificate, err := tls.LoadX509KeyPair(*certFile, *keyFile)
			if err != nil {
				return err
			}

			tlsConfig = &tls.Config{
				InsecureSkipVerify: true,
				Certificates:       []tls.Certificate{certificate},
			}
		} else if *generateCert {
			certificate, err := crypto.GenerateCertificate()
			if err != nil {
				return err
			}

			tlsConfig = &tls.Config{
				InsecureSkipVerify: true,
				Certificates:       []tls.Certificate{certificate},
			}
		}

		if err := godotenv.Load(); err != nil {
			logger.Infof("Could not load .env file: %v", err)
		}

		agent, err := app.NewAgent(group.Ctx(), tlsConfig)
		if err == nil {
			consumer, err_ := playnite.NewGpuMetricsConsumer(agent)
			err = err_
			if err != nil {
				logger.Warning(err)
			}

			agent.GpuMetricsProvider.AddConsumer(consumer)
			agent.GpuMetricsProvider.AddConsumer(prometheus.NewGpuMetricsConsumer())

			err = agent.ConnectToController(group, tlsConfig)
			if err == nil {
				group.Go("Agent", agent)
			} else {
				group.Cancel()
			}
		}

		return err
	})

	if err != nil {
		os.Exit(appmain.ExitFailure)
	}
}
