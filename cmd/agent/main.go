/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"

	"github.com/Juice-Labs/Juice-Labs/cmd/agent/app"
	"github.com/Juice-Labs/Juice-Labs/cmd/agent/playnite"
	"github.com/Juice-Labs/Juice-Labs/cmd/internal/build"
	"github.com/Juice-Labs/Juice-Labs/pkg/appmain"
	"github.com/Juice-Labs/Juice-Labs/pkg/crypto"
	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
)

var (
	certFile     = flag.String("cert-file", "", "")
	keyFile      = flag.String("key-file", "", "")
	generateCert = flag.Bool("generate-cert", false, "Generates a certificate for https")
	disableTls   = flag.Bool("disable-tls", true, "")
)

func main() {
	appmain.Run("Juice Agent", build.Version, func(ctx context.Context) error {
		var tlsConfig *tls.Config

		var err error
		if !*disableTls {
			var certificate tls.Certificate
			if *certFile != "" && *keyFile != "" {
				certificate, err = tls.LoadX509KeyPair(*certFile, *keyFile)
			} else if *generateCert {
				certificate, err = crypto.GenerateCertificate()
			} else {
				err = errors.New("https is required, use both --cert-file and --key-file or --generate-cert")
			}

			if err == nil {
				tlsConfig = &tls.Config{
					Certificates: []tls.Certificate{certificate},
				}
			}
		}

		if err == nil {
			agent, err := app.NewAgent(ctx, tlsConfig)
			if err == nil {
				consumer, err := playnite.NewGpuMetricsConsumer(agent)
				if err != nil {
					logger.Warning(err)
				}

				agent.GpuMetricsProvider.AddConsumer(consumer)

				err = agent.ConnectToController()
				if err == nil {
					agent.Start()
				} else {
					agent.Cancel()
				}

				return errors.Join(err, agent.Wait())
			}

			return err
		}

		return nil
	})
}
