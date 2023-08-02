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
	"net/http"
	"os"
	"strings"

	"github.com/Juice-Labs/Juice-Labs/cmd/controller/backend"
	"github.com/Juice-Labs/Juice-Labs/cmd/controller/frontend"
	"github.com/Juice-Labs/Juice-Labs/cmd/controller/prometheus"
	"github.com/Juice-Labs/Juice-Labs/cmd/controller/storage"
	"github.com/Juice-Labs/Juice-Labs/cmd/controller/storage/memdb"
	"github.com/Juice-Labs/Juice-Labs/cmd/controller/storage/postgres"
	"github.com/gorilla/mux"

	"github.com/joho/godotenv"

	"github.com/Juice-Labs/Juice-Labs/cmd/internal/build"
	"github.com/Juice-Labs/Juice-Labs/pkg/appmain"
	"github.com/Juice-Labs/Juice-Labs/pkg/crypto"
	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
	"github.com/Juice-Labs/Juice-Labs/pkg/server"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
)

var (
	certFile     = flag.String("cert-file", "", "")
	keyFile      = flag.String("key-file", "", "")
	generateCert = flag.Bool("generate-cert", false, "Generates a certificate for https")

	enableFrontend   = flag.Bool("frontend", false, "")
	enableBackend    = flag.Bool("backend", false, "")
	enablePrometheus = flag.Bool("prometheus", false, "")

	address           = flag.String("address", "0.0.0.0:8080", "The IP address and port to use for listening for client connections")
	prometheusAddress = flag.String("prometheus-address", "0.0.0.0:9090", "The IP address and port to use for listening for Prometheus connections")

	psqlConnection         = flag.String("psql-connection", "", "See https://pkg.go.dev/github.com/lib/pq#hdr-Connection_String_Parameters")
	psqlConnectionFromFile = flag.String("psql-connection-from-file", "", "See https://pkg.go.dev/github.com/lib/pq#hdr-Connection_String_Parameters")

	mainServer       *server.Server
	prometheusServer *server.Server
)

func openStorage(ctx context.Context) (storage.Storage, error) {
	psqlConnectionFromEnv, psqlConnectionFromEnvPresent := os.LookupEnv("PSQL_CONNECTION")
	if len(*psqlConnection) > 0 || len(*psqlConnectionFromFile) > 0 || psqlConnectionFromEnvPresent {
		count := 0
		if len(*psqlConnection) > 0 {
			count++
		}
		if len(*psqlConnectionFromFile) > 0 {
			count++
		}
		if psqlConnectionFromEnvPresent {
			count++
		}
		if count > 1 {
			return nil, errors.New("--psql-connection, --psql-connection-from-file and environment variable PSQL_CONNECTION are mutually exclusive")
		}

		connection := ""
		if len(*psqlConnection) > 0 {
			connection = *psqlConnection
		} else if len(*psqlConnectionFromFile) > 0 {
			text, err := ioutil.ReadFile(*psqlConnectionFromFile)
			if err != nil {
				return nil, fmt.Errorf("unable to read file %s, %v", *psqlConnectionFromFile, err)
			}

			connection = strings.TrimSpace(string(text))
		} else {
			connection = psqlConnectionFromEnv
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

		var certificate tls.Certificate
		if *certFile != "" && *keyFile != "" {
			certificate, err = tls.LoadX509KeyPair(*certFile, *keyFile)
		} else if *generateCert {
			certificate, err = crypto.GenerateCertificate()
		}

		if err == nil {
			tlsConfig = &tls.Config{
				Certificates: []tls.Certificate{certificate},
			}
		}

		if err := godotenv.Load(); err != nil {
			logger.Debugf("unable to load the .env file: %v", err)
		}

		if err == nil {
			if *enableFrontend || *enableBackend {
				mainServer, err = server.NewServer(*address, tlsConfig)
				if err == nil {
					mainServer.AddCreateEndpoint(func(group task.Group, router *mux.Router) error {
						router.Methods("GET").Path("/health").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							w.WriteHeader(http.StatusOK)
						})

						return nil
					})
				}

				if err == nil {
					if *enableFrontend {
						logger.Infof("Starting frontend on %s", *address)

						frontend, err_ := frontend.NewFrontend(mainServer, storage)
						err = err_
						if err == nil {
							group.Go("Frontend", frontend)
						}
					}
				}

				if err == nil {
					if *enableBackend {
						logger.Infof("Starting backend on %s", *address)

						group.Go("Backend", backend.NewBackend(storage))
					}
				}

				if err == nil {
					group.Go("Main Server", mainServer)
				}
			}
		}

		if err == nil {
			if *enablePrometheus {
				prometheusServer, err = server.NewServer(*prometheusAddress, tlsConfig)
				if err == nil {
					prometheusServer.AddCreateEndpoint(func(group task.Group, router *mux.Router) error {
						router.Methods("GET").Path("/health").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							w.WriteHeader(http.StatusOK)
						})

						return nil
					})

					logger.Infof("Starting prometheus on %s", *prometheusAddress)

					group.Go("Prometheus", prometheus.NewFrontend(prometheusServer, storage))
					group.Go("Prometheus Server", prometheusServer)
				}
			}
		}

		if err != nil {
			group.Cancel()
		}

		return err
	})
}
