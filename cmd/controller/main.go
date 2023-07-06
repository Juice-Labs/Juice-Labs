/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"

	"github.com/Juice-Labs/Juice-Labs/cmd/controller/backend"
	"github.com/Juice-Labs/Juice-Labs/cmd/controller/frontend"
	"github.com/Juice-Labs/Juice-Labs/cmd/controller/storage"
	"github.com/Juice-Labs/Juice-Labs/cmd/controller/storage/memdb"
	"github.com/Juice-Labs/Juice-Labs/cmd/controller/storage/postgres"
	"github.com/Juice-Labs/Juice-Labs/cmd/internal/build"
	"github.com/Juice-Labs/Juice-Labs/pkg/appmain"
	"github.com/Juice-Labs/Juice-Labs/pkg/crypto"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
)

var (
	certFile     = flag.String("cert-file", "", "")
	keyFile      = flag.String("key-file", "", "")
	generateCert = flag.Bool("generate-cert", false, "Generates a certificate for https")
	disableTls   = flag.Bool("disable-tls", true, "")

	enableFrontend = flag.Bool("enable-frontend", false, "")
	enableBackend  = flag.Bool("enable-backend", false, "")

	useMemdb = flag.Bool("use-memdb", false, "")
)

func openStorage(ctx context.Context) (storage.Storage, error) {
	if *useMemdb {
		return memdb.OpenStorage(ctx)
	}

	return postgres.OpenStorage(ctx)
}

func main() {
	appmain.Run("Juice Controller", build.Version, func(group task.Group) error {
		var err error

		storage, err := openStorage(group.Ctx())
		if err == nil {
			group.GoFn("Storage Close", func(group task.Group) error {
				<-group.Ctx().Done()
				return storage.Close()
			})
		}

		if *enableFrontend {
			var tlsConfig *tls.Config

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
				frontend, err := frontend.NewFrontend(tlsConfig, storage)
				if err == nil {
					group.Go("Frontend", frontend)
				}
			}
		}

		if *enableBackend {
			if err == nil {
				group.Go("Backend", backend.NewBackend(storage))
			}
		}

		if err != nil {
			group.Cancel()
		}

		return err
	})
}
