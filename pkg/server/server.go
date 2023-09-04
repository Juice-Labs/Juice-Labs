/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package server

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/rs/cors"

	"github.com/Juice-Labs/Juice-Labs/pkg/errors"
	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
)

var (
	ErrInvalidPort = errors.New("server: address does not contain a valid port")
)

type EndpointHandlerFn = func(group task.Group, w http.ResponseWriter, r *http.Request)

type Endpoint struct {
	Name    string
	Methods []string
	Path    string
	Handler EndpointHandlerFn
}

type Server struct {
	url url.URL

	port int

	root      *mux.Router
	tlsConfig *tls.Config

	endpoints []Endpoint
}

func NewServer(address string, tlsConfig *tls.Config) (*Server, error) {
	url := url.URL{
		Host: address,
	}

	portStr := url.Port()
	if portStr == "" {
		if tlsConfig != nil {
			portStr = "443"
		} else {
			portStr = "80"
		}
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, ErrInvalidPort
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

	root := mux.NewRouter().StrictSlash(true)
	root.Use(logger.Middleware, cors.Handler)

	server := &Server{
		url:       url,
		port:      port,
		root:      root,
		tlsConfig: tlsConfig,
	}

	server.AddEndpointFunc("GET", "/health", func(group task.Group, w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	return server, nil
}

func (server *Server) Port() int {
	return server.port
}

func (server *Server) AddEndpointFunc(method string, path string, fn EndpointHandlerFn) {
	server.AddEndpoint(Endpoint{
		Methods: []string{method},
		Path:    path,
		Handler: fn,
	})
}

func (server *Server) AddNamedEndpointFunc(name string, method string, path string, fn EndpointHandlerFn) {
	server.AddEndpoint(Endpoint{
		Name:    name,
		Methods: []string{method},
		Path:    path,
		Handler: fn,
	})
}

func (server *Server) AddEndpointHandler(method string, path string, handler http.Handler) {
	server.AddEndpoint(Endpoint{
		Methods: []string{method},
		Path:    path,
		Handler: func(group task.Group, w http.ResponseWriter, r *http.Request) {
			handler.ServeHTTP(w, r)
		},
	})
}

func (server *Server) AddEndpoint(endpoint Endpoint) {
	server.endpoints = append(server.endpoints, endpoint)
}

func (server *Server) RemoveEndpointByName(name string) {
	if name != "" {
		for index, endpoint := range server.endpoints {
			if endpoint.Name == name {
				if (index + 1) == len(server.endpoints) {
					server.endpoints = server.endpoints[0:index]
				} else {
					server.endpoints = append(server.endpoints[0:index], server.endpoints[index+1:]...)
				}

				break
			}
		}
	}
}

func (server *Server) Run(group task.Group) error {
	for _, endpoint := range server.endpoints {
		endpoint := endpoint // Creates a new variable in this for loop for the following closure to capture it correctly
		server.root.Methods(endpoint.Methods...).Path(endpoint.Path).HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			endpoint.Handler(group, w, r)
		})
	}

	httpServer := http.Server{
		BaseContext: func(_ net.Listener) context.Context {
			return group.Ctx()
		},
		Addr:      server.url.Host,
		Handler:   server.root,
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
