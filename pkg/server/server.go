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
	"github.com/rs/cors"

	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
)

type CreateEndpointFn = func(group task.Group, router *mux.Router) error

type Server struct {
	url url.URL

	port int

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
		port:            port,
		tlsConfig:       tlsConfig,
		createEndpoints: map[string]CreateEndpointFn{},
	}, nil
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

	// Enable CORS
	cors := cors.New(cors.Options{
		AllowedOrigins: []string{"http://localhost:3000", "https://juiceweb.vercel.app", "http://wails.localhost:34115", "wails://wails", "http://wails.localhost"},
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodOptions,
			http.MethodHead,
		},

		AllowedHeaders: []string{
			"*",
		},
	})

	loggerRouter := mux.NewRouter().StrictSlash(true)
	loggerRouter.Use(logger.Middleware)
	loggerRouter.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cors.ServeHTTP(w, r, func(w http.ResponseWriter, r *http.Request) {
			router.ServeHTTP(w, r)
		})
	})

	httpServer := http.Server{
		BaseContext: func(_ net.Listener) context.Context {
			return group.Ctx()
		},
		Addr:      server.url.Host,
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
