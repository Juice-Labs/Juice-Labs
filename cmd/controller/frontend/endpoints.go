/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package frontend

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/Juice-Labs/Juice-Labs/cmd/internal/build"
	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
	pkgnet "github.com/Juice-Labs/Juice-Labs/pkg/net"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
	"github.com/Juice-Labs/Juice-Labs/pkg/server"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
)

func (frontend *Frontend) initializeEndpoints(server *server.Server) {
	server.AddEndpointFunc("GET", "/status", frontend.getStatusFormerEp)
	server.AddEndpointFunc("GET", "/v1/status", frontend.getStatusEp)
	server.AddEndpointFunc("POST", "/v1/register/agent", frontend.registerAgentEp)
	server.AddEndpointFunc("GET", "/v1/agent/{id}", frontend.getAgentEp)
	server.AddEndpointFunc("PUT", "/v1/agent/{id}", frontend.updateAgentEp)
	server.AddEndpointFunc("GET", "/v1/agents", frontend.getAgentsEp)
	server.AddEndpointFunc("POST", "/v1/request/session", frontend.requestSessionEp)
	server.AddEndpointFunc("GET", "/v1/session/{id}", frontend.cancelSessionEp)
	server.AddEndpointFunc("DELETE", "/v1/session/{id}", frontend.getSessionEp)
}

func (frontend *Frontend) getStatusEp(group task.Group, w http.ResponseWriter, r *http.Request) {
	err := pkgnet.Respond(w, http.StatusOK, restapi.Status{
		State:    "Active",
		Version:  build.Version,
		Hostname: frontend.hostname,
	})

	if err != nil {
		err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
		logger.Error(err)
	}
}

func (frontend *Frontend) registerAgentEp(group task.Group, w http.ResponseWriter, r *http.Request) {
	agent, err := pkgnet.ReadRequestBody[restapi.Agent](r)
	if err != nil {
		err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
		logger.Error(err)
		return
	}

	id, err := frontend.registerAgent(agent)
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

func (frontend *Frontend) getAgentEp(group task.Group, w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	agent, err := frontend.getAgentById(id)
	if err != nil {
		err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
		logger.Error(err)
		return
	}

	pkgnet.Respond(w, http.StatusOK, agent)
}

func (frontend *Frontend) getAgentsEp(group task.Group, w http.ResponseWriter, r *http.Request) {
	agents, err := frontend.getAgents()
	if err != nil {
		err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
		logger.Error(err)
		return
	}

	pkgnet.Respond(w, http.StatusOK, agents)
}

func (frontend *Frontend) updateAgentEp(group task.Group, w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	update, err := pkgnet.ReadRequestBody[restapi.AgentUpdate](r)
	if err != nil {
		err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
		logger.Error(err)
		return
	}

	if update.Id != id {
		err = fmt.Errorf("/v1/agent/%s: ids do not match", id)
		err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusBadRequest, err.Error()))
		logger.Error(err)
		return
	}

	err = frontend.updateAgent(group.Ctx(), update)
	if err != nil {
		err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
		logger.Error(err)
		return
	}

	pkgnet.RespondEmpty(w, http.StatusOK)
}

func (frontend *Frontend) requestSessionEp(group task.Group, w http.ResponseWriter, r *http.Request) {
	sessionRequirements, err := pkgnet.ReadRequestBody[restapi.SessionRequirements](r)
	if err != nil {
		err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
		logger.Error(err)
		return
	}

	id, err := frontend.requestSession(sessionRequirements)
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

func (frontend *Frontend) cancelSessionEp(group task.Group, w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	err := frontend.cancelSession(id)
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

func (frontend *Frontend) getSessionEp(group task.Group, w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	session, err := frontend.getSessionById(id)
	if err != nil {
		err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
		logger.Error(err)
		return
	}

	err = pkgnet.Respond(w, http.StatusOK, session)
	if err != nil {
		logger.Error(err)
	}
}
