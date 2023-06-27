/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package server

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gorilla/mux"

	"github.com/Juice-Labs/Juice-Labs//pkg/logger"
	"github.com/Juice-Labs/Juice-Labs//pkg/task"
)

var (
	ErrMissingPort            = errors.New("server: missing address port")
	ErrEndpointCreationFailed = errors.New("server: endpoint creation failed")
)

type CreateEndpointFn = func(router *mux.Router) error

type Server struct {
	url       url.URL
	tlsConfig *tls.Config

	createEndpoints          map[string]CreateEndpointFn
	immutableCreateEndpoints []CreateEndpointFn
}

func NewServer(address string, tlsConfig *tls.Config) *Server {
	return &Server{
		url: url.URL{
			Host: address,
		},
		tlsConfig:       tlsConfig,
		createEndpoints: map[string]CreateEndpointFn{},
	}
}

func (server *Server) Address() string {
	return server.url.Host
}

func (server *Server) Port() (int, error) {
	portStr := server.url.Port()
	if portStr == "" {
		return 0, ErrMissingPort
	}

	return strconv.Atoi(portStr)
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
			err = errors.Join(err, createEndpoint(router))
		}
	}

	for _, createEndpoint := range server.immutableCreateEndpoints {
		if createEndpoint != nil {
			err = errors.Join(err, createEndpoint(router))
		}
	}

	if err != nil {
		return errors.Join(err, ErrEndpointCreationFailed)
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

	group.GoFn(func(group task.Group) error {
		if server.tlsConfig != nil {
			return httpServer.ListenAndServeTLS("", "")
		} else {
			return httpServer.ListenAndServe()
		}
	})

	group.GoFn(func(group task.Group) error {
		<-group.Ctx().Done()

		return httpServer.Shutdown(group.Ctx())
	})

	return nil
}
