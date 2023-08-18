/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package app

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/Juice-Labs/Juice-Labs/cmd/agent/prometheus"
	"github.com/Juice-Labs/Juice-Labs/cmd/internal/build"
	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
	"github.com/Juice-Labs/Juice-Labs/pkg/middleware"
	pkgnet "github.com/Juice-Labs/Juice-Labs/pkg/net"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
	"github.com/Juice-Labs/Juice-Labs/pkg/utilities"
)

const (
	RequestSessionName = "RequestSession"
)

func (agent *Agent) initializeEndpoints() {
	agent.Server.AddCreateEndpoint(agent.getStatusEp)
	agent.Server.SetCreateEndpoint(RequestSessionName, agent.requestSessionEp)
	agent.Server.AddCreateEndpoint(agent.getSessionEp)
	agent.Server.AddCreateEndpoint(agent.cancelSessionEp)
	agent.Server.AddCreateEndpoint(agent.connectSessionEp)

	prometheus.InitializeEndpoints(agent.Server)
}

func (agent *Agent) getStatusEp(group task.Group, router *mux.Router) error {
	router.Methods("GET").Path("/v1/status").HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			err := pkgnet.Respond(w, http.StatusOK, restapi.Status{
				State:    "Active",
				Version:  build.Version,
				Hostname: agent.Hostname,
			})

			if err != nil {
				err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
				logger.Error(err)
			}
		})
	return nil
}

func (agent *Agent) requestSessionEp(group task.Group, router *mux.Router) error {
	// TODO: Validate create:sessions claim
	router.Handle("/v1/request/session", middleware.EnsureValidToken()(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			sessionRequirements, err := pkgnet.ReadRequestBody[restapi.SessionRequirements](r)
			if err != nil {
				err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
				logger.Error(err)
				return
			}

			id, err := agent.requestSession(group, sessionRequirements)
			if err != nil {
				err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
				logger.Error(err)
				return
			}

			err = pkgnet.RespondWithString(w, http.StatusOK, id)
			if err != nil {
				logger.Error(err)
			}
		}))).Methods("POST")
	return nil
}

func (agent *Agent) getSessionEp(group task.Group, router *mux.Router) error {
	// TODO: Validate read:sessions claim
	router.Handle("/v1/session/{id}", middleware.EnsureValidToken()(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			id := mux.Vars(r)["id"]

			reference, err := agent.getSession(id)
			if err != nil {
				err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
				logger.Error(err)
				return
			}
			defer reference.Release()

			err = pkgnet.Respond(w, http.StatusOK, reference.Object.Session())
			if err != nil {
				logger.Error(err)
			}
		}))).Methods("GET")
	return nil
}

func (agent *Agent) cancelSessionEp(group task.Group, router *mux.Router) error {
	// TODO: Validate write:sessions claim
	router.Handle("/v1/session/{id}", middleware.EnsureValidToken()(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			id := mux.Vars(r)["id"]

			err := agent.cancelSession(id)
			if err != nil {
				err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
				logger.Error(err)
				return
			}

			err = pkgnet.Respond(w, http.StatusOK, fmt.Sprintf("Session %s cancelled", id))
			if err != nil {
				logger.Error(err)
			}
		}))).Methods("DELETE")
	return nil
}

func (agent *Agent) connectSessionEp(group task.Group, router *mux.Router) error {
	// TODO: Validate connect:sessions claim
	router.Handle("/v1/connect/session/{id}", middleware.EnsureValidToken()(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			id := mux.Vars(r)["id"]
			connectionData, err := pkgnet.ReadRequestBody[restapi.ConnectionData](r)
			if err != nil {
				err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
				logger.Error(err)
				return
			}

			reference, err := agent.getSession(id)
			if err != nil {
				err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
				logger.Error(err)
				return
			}
			defer reference.Release()

			var conn net.Conn

			hijacker, err := utilities.Cast[http.Hijacker](w)
			if err == nil {
				var buf *bufio.ReadWriter
				conn, buf, err = hijacker.Hijack()
				if err == nil {
					if buf.Reader.Buffered() > 0 {
						err = fmt.Errorf("/v1/connect/session/%s: hijacked connection has buffered data", id)
					}
				}
			}

			if err != nil {
				err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
				logger.Error(err)
				return
			}

			err = agent.connect(group, connectionData, id, conn)
			if err != nil {
				logger.Error(err)
			}
		}))).Methods("POST")
	return nil
}
