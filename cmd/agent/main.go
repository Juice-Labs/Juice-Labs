/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package main

import (
	"crypto/tls"
	"flag"

	"github.com/Juice-Labs/Juice-Labs/cmd/agent/app"
	"github.com/Juice-Labs/Juice-Labs/cmd/agent/playnite"
	"github.com/Juice-Labs/Juice-Labs/cmd/agent/prometheus"
	"github.com/Juice-Labs/Juice-Labs/cmd/internal/build"
	"github.com/Juice-Labs/Juice-Labs/pkg/appmain"
	"github.com/Juice-Labs/Juice-Labs/pkg/crypto"
	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
	"github.com/joho/godotenv"
)

var (
	certFile     = flag.String("cert-file", "", "")
	keyFile      = flag.String("key-file", "", "")
	generateCert = flag.Bool("generate-cert", false, "Generates a certificate for https")
)

func main() {
	appmain.Run("Juice Agent", build.Version, func(group task.Group) error {
		var tlsConfig *tls.Config

		var err error
		var certificates []tls.Certificate
		if *certFile != "" && *keyFile != "" {
			certificate, err_ := tls.LoadX509KeyPair(*certFile, *keyFile)
			err = err_
			if err == nil {
				certificates = append(certificates, certificate)
			}
		} else if *generateCert {
			certificate, err_ := crypto.GenerateCertificate()
			err = err_
			if err == nil {
				certificates = append(certificates, certificate)
			}
		}

		if certificates != nil {
			tlsConfig = &tls.Config{
				Certificates: certificates,
			}
		}

		if err := godotenv.Load(); err != nil {
			logger.Infof("Could not load .env file: %v", err)
		}

		if err == nil {
			agent, err := app.NewAgent(group.Ctx(), tlsConfig)
			if err == nil {
				consumer, err_ := playnite.NewGpuMetricsConsumer(agent)
				err = err_
				if err != nil {
					logger.Warning(err)
				}

				agent.GpuMetricsProvider.AddConsumer(consumer)
				agent.GpuMetricsProvider.AddConsumer(prometheus.NewGpuMetricsConsumer())

				err = agent.ConnectToController(group)
				if err == nil {
					group.Go("Agent", agent)
				} else {
					group.Cancel()
				}
			}

			return err
		}

		return nil
	})
}
