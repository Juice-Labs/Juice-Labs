/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gorilla/mux"

	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
)

type CreateEndpointFn = func(group task.Group, router *mux.Router) error

type Server struct {
	url url.URL

	address string // Takes the form ":<port>"
	port    int

	tlsConfig *tls.Config

	createEndpoints          map[string]CreateEndpointFn
	immutableCreateEndpoints []CreateEndpointFn
}

func NewServer(address string, tlsConfig *tls.Config) (*Server, error) {
	url := url.URL{
		Host: address,
	}

	portStr := url.Port()
	if portStr == "" {
		return nil, errors.New("NewServer: missing address port")
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("NewServer: address does not contain a valid port")
	}

	return &Server{
		url:             url,
		address:         fmt.Sprintf(":%d", port),
		port:            port,
		tlsConfig:       tlsConfig,
		createEndpoints: map[string]CreateEndpointFn{},
	}, nil
}

func (server *Server) Address() string {
	return server.address
}

func (server *Server) Port() int {
	return server.port
}

func (server *Server) AddCreateEndpoint(fn CreateEndpointFn) {
	server.immutableCreateEndpoints = append(server.immutableCreateEndpoints, fn)
}

func (server *Server) SetCreateEndpoint(name string, fn CreateEndpointFn) {
	server.createEndpoints[name] = fn
}

func (server *Server) Run(group task.Group) error {
	router := mux.NewRouter().StrictSlash(true)

	var err error
	for _, createEndpoint := range server.createEndpoints {
		if createEndpoint != nil {
			err = errors.Join(err, createEndpoint(group, router))
		}
	}

	for _, createEndpoint := range server.immutableCreateEndpoints {
		if createEndpoint != nil {
			err = errors.Join(err, createEndpoint(group, router))
		}
	}

	if err != nil {
		return fmt.Errorf("Server.Run: endpoint creation failed with %s", err)
	}

	loggerRouter := mux.NewRouter().StrictSlash(true)
	loggerRouter.Use(logger.Middleware)
	loggerRouter.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		router.ServeHTTP(w, r)
	})

	httpServer := http.Server{
		BaseContext: func(_ net.Listener) context.Context {
			return group.Ctx()
		},
		Addr:      server.Address(),
		Handler:   loggerRouter,
		TLSConfig: server.tlsConfig,
	}

	group.GoFn("HTTP Listen", func(group task.Group) error {
		if server.tlsConfig != nil {
			return httpServer.ListenAndServeTLS("", "")
		} else {
			return httpServer.ListenAndServe()
		}
	})

	group.GoFn("HTTP Shutdown", func(group task.Group) error {
		<-group.Ctx().Done()

		return httpServer.Shutdown(group.Ctx())
	})

	return nil
}
