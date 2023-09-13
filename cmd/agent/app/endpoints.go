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
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/Juice-Labs/Juice-Labs/cmd/internal/build"
	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
	pkgnet "github.com/Juice-Labs/Juice-Labs/pkg/net"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
	"github.com/Juice-Labs/Juice-Labs/pkg/utilities"
)

const (
	RequestSessionName = "RequestSession"
)

func (agent *Agent) initializeEndpoints() {
	agent.Server.AddEndpointFunc("GET", "/v1/status", agent.getStatusEp)
	agent.Server.AddNamedEndpointFunc(RequestSessionName, "POST", "/v1/request/session", agent.requestSessionEp)
	agent.Server.AddEndpointFunc("GET", "/v1/session/{id}", agent.getSessionEp)
	agent.Server.AddEndpointFunc("DELETE", "/v1/session/{id}", agent.cancelSessionEp)
	agent.Server.AddEndpointFunc("POST", "/v1/connect/session/{id}", agent.connectSessionEp)

	agent.Server.AddEndpointHandler("GET", "/metrics", promhttp.Handler())
}

func (agent *Agent) getStatusEp(w http.ResponseWriter, r *http.Request) {
	err := pkgnet.Respond(w, http.StatusOK, restapi.Status{
		State:    "Active",
		Version:  build.Version,
		Hostname: agent.Hostname,
	})

	if err != nil {
		err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
		logger.Error(err)
	}
}

func (agent *Agent) requestSessionEp(w http.ResponseWriter, r *http.Request) {
	sessionRequirements, err := pkgnet.ReadRequestBody[restapi.SessionRequirements](r)
	if err != nil {
		err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
		logger.Error(err)
		return
	}

	id, err := agent.requestSession(sessionRequirements)
	if err != nil {
		err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
		logger.Error(err)
		return
	}

	err = pkgnet.RespondWithString(w, http.StatusOK, id)
	if err != nil {
		logger.Error(err)
	}
}

func (agent *Agent) getSessionEp(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	session, err := agent.getSession(id)
	if err != nil {
		err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
		logger.Error(err)
		return
	}

	err = pkgnet.Respond(w, http.StatusOK, session.Session())
	if err != nil {
		logger.Error(err)
	}
}

func (agent *Agent) cancelSessionEp(w http.ResponseWriter, r *http.Request) {
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
}

func (agent *Agent) connectSessionEp(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	connectionData, err := pkgnet.ReadRequestBody[restapi.ConnectionData](r)
	if err != nil {
		err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
		logger.Error(err)
		return
	}

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

	err = agent.connect(id, connectionData, conn)
	if err != nil {
		logger.Error(err)
	}
}
