/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package app

import (
	"crypto/tls"
	"flag"
	"os"

	"github.com/Juice-Labs/Juice-Labs/pkg/backend"
	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
	"github.com/Juice-Labs/Juice-Labs/pkg/server"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
)

var (
	address = flag.String("address", "0.0.0.0:43210", "The IP address and port to use for listening for client connections")
)

type Controller struct {
	hostname string

	server  *server.Server
	backend *backend.Backend
}

func NewController(tlsConfig *tls.Config) (*Controller, error) {
	if tlsConfig == nil {
		logger.Warning("TLS is disabled, data will be unencrypted")
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	controller := &Controller{
		hostname: hostname,
		server:   server.NewServer(*address, tlsConfig),
	}

	controller.initializeEndpoints()

	return controller, nil
}

func (controller *Controller) Run(group task.Group) error {
	backend, err := backend.NewBackend(group.Ctx())
	if err != nil {
		return err
	}

	controller.backend = backend

	group.Go(controller.server)
	group.GoFn(func(group task.Group) error {
		<-group.Ctx().Done()
		return controller.backend.Close()
	})

	return nil
}
