/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/Juice-Labs/Juice-Labs/cmd/controller/backend"
	"github.com/Juice-Labs/Juice-Labs/cmd/controller/frontend"
	"github.com/Juice-Labs/Juice-Labs/cmd/controller/prometheus"
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

	enableFrontend   = flag.Bool("frontend", false, "")
	enableBackend    = flag.Bool("backend", false, "")
	enablePrometheus = flag.Bool("prometheus", false, "")

	psqlConnection         = flag.String("psql-connection", "", "See https://pkg.go.dev/github.com/lib/pq#hdr-Connection_String_Parameters")
	psqlConnectionFromFile = flag.String("psql-connection-from-file", "", "See https://pkg.go.dev/github.com/lib/pq#hdr-Connection_String_Parameters")
)

func openStorage(ctx context.Context) (storage.Storage, error) {
	if len(*psqlConnection) > 0 || len(*psqlConnectionFromFile) > 0 {
		if len(*psqlConnection) > 0 && len(*psqlConnectionFromFile) > 0 {
			return nil, errors.New("--psql-connection and --psql-connection-from-file are mutually exclusive, one or the other but not both")
		}

		connection := ""
		if len(*psqlConnection) > 0 {
			connection = *psqlConnection
		} else {
			text, err := ioutil.ReadFile(*psqlConnectionFromFile)
			if err != nil {
				return nil, fmt.Errorf("unable to read file %s, %v", *psqlConnectionFromFile, err)
			}

			connection = strings.TrimSpace(string(text))
		}

		return postgres.OpenStorage(ctx, connection)
	}

	return memdb.OpenStorage(ctx)
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

		var tlsConfig *tls.Config

		if (*enableFrontend || *enablePrometheus) && !*disableTls {
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

		if *enableFrontend {
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

		if *enablePrometheus {
			if err == nil {
				frontend, err := prometheus.NewFrontend(tlsConfig, storage)
				if err == nil {
					group.Go("Prometheus", frontend)
				}
			}
		}

		if err != nil {
			group.Cancel()
		}

		return err
	})
}
