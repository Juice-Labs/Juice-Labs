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
	"github.com/Juice-Labs/Juice-Labs/pkg/middleware"
	"github.com/Juice-Labs/Juice-Labs/pkg/sentry"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"

	sentryhttp "github.com/getsentry/sentry-go/http"
)

var (
	ErrInvalidPort = errors.New("server: address does not contain a valid port")
)

type Endpoint struct {
	Name        string
	Methods     []string
	Queries     []string
	Path        string
	Handler     http.Handler
	RequireAuth bool
}

type Server struct {
	url url.URL

	port int

	root      *mux.Router
	handler   http.Handler
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

	if sentry.Enabled() {
		sentryHandler := sentryhttp.New(sentryhttp.Options{
			Repanic: true,
		})

		root.Use(sentryHandler.Handle)
	}

	root.Use(logger.Middleware)
	handler := cors.Handler(root)

	server := &Server{
		url:       url,
		port:      port,
		root:      root,
		handler:   handler,
		tlsConfig: tlsConfig,
	}

	server.AddEndpointFunc("GET", "/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}, false)

	return server, nil
}

func (server *Server) Port() int {
	return server.port
}

func (server *Server) AddEndpointFunc(method string, path string, fn http.HandlerFunc, requireAuth bool) {
	server.AddEndpoint(Endpoint{
		Methods:     []string{method},
		Path:        path,
		Handler:     fn,
		RequireAuth: requireAuth,
	})
}
func (server *Server) AddEndpointFuncWithQuery(method string, path string, fn http.HandlerFunc, requireAuth bool, queries []string) {
	server.AddEndpoint(Endpoint{
		Methods:     []string{method},
		Path:        path,
		Handler:     fn,
		RequireAuth: requireAuth,
		Queries:     queries,
	})
}

func (server *Server) AddNamedEndpointFunc(name string, method string, path string, fn http.HandlerFunc, requireAuth bool) {
	server.AddEndpoint(Endpoint{
		Name:        name,
		Methods:     []string{method},
		Path:        path,
		Handler:     fn,
		RequireAuth: requireAuth,
	})
}

func (server *Server) AddEndpointHandler(method string, path string, handler http.Handler, requireAuth bool) {
	server.AddEndpoint(Endpoint{
		Methods:     []string{method},
		Path:        path,
		Handler:     handler,
		RequireAuth: requireAuth,
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

		route := server.root.Methods(endpoint.Methods...).Path(endpoint.Path)

		if endpoint.RequireAuth {
			handlerWithAuth := middleware.EnsureValidToken()(endpoint.Handler)
			route.Handler(handlerWithAuth)
		} else {
			route.Handler(endpoint.Handler)
		}

		if len(endpoint.Queries) > 0 {
			route.Queries(endpoint.Queries...)
		}

	}

	httpServer := http.Server{
		BaseContext: func(_ net.Listener) context.Context {
			return group.Ctx()
		},
		Addr:      server.url.Host,
		Handler:   server.handler,
		TLSConfig: server.tlsConfig,
	}

	group.GoFn("HTTP Listen", func(group task.Group) error {
		var err error
		if server.tlsConfig != nil {
			err = httpServer.ListenAndServeTLS("", "")
		} else {
			err = httpServer.ListenAndServe()
		}
		if err == http.ErrServerClosed {
			return nil
		}
		
		return err
	})

	group.GoFn("HTTP Shutdown", func(group task.Group) error {
		<-group.Ctx().Done()

		return httpServer.Shutdown(group.Ctx())
	})

	return nil
}
