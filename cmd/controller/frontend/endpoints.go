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
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
)

func (frontend *Frontend) initializeEndpoints() {
	frontend.server.AddCreateEndpoint(func(group task.Group, router *mux.Router) error {
		router.Methods("GET").Path("/health").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		return nil
	})

	frontend.server.AddCreateEndpoint(frontend.getStatusFormer)
	frontend.server.AddCreateEndpoint(frontend.getStatusEp)
	frontend.server.AddCreateEndpoint(frontend.registerAgentEp)
	frontend.server.AddCreateEndpoint(frontend.getAgentEp)
	frontend.server.AddCreateEndpoint(frontend.getAgentsEp)
	frontend.server.AddCreateEndpoint(frontend.updateAgentEp)
	frontend.server.AddCreateEndpoint(frontend.requestSessionEp)
	frontend.server.AddCreateEndpoint(frontend.getSessionEp)
}

func (frontend *Frontend) getStatusEp(group task.Group, router *mux.Router) error {
	router.Methods("GET").Path("/v1/status").HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			err := pkgnet.Respond(w, http.StatusOK, restapi.Status{
				State:    "Active",
				Version:  build.Version,
				Hostname: frontend.hostname,
			})

			if err != nil {
				err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
				logger.Error(err)
			}
		})
	return nil
}

func (frontend *Frontend) registerAgentEp(group task.Group, router *mux.Router) error {
	router.Methods("POST").Path("/v1/register/agent").HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
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
		})
	return nil
}

func (frontend *Frontend) getAgentEp(group task.Group, router *mux.Router) error {
	router.Methods("GET").Path("/v1/agent/{id}").HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			id := mux.Vars(r)["id"]

			agent, err := frontend.getAgentById(id)
			if err != nil {
				err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
				logger.Error(err)
				return
			}

			pkgnet.Respond(w, http.StatusOK, agent)
		})
	return nil
}

func (frontend *Frontend) getAgentsEp(group task.Group, router *mux.Router) error {
	router.Methods("GET").Path("/v1/agents").HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			agents, err := frontend.getAgents()
			if err != nil {
				err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
				logger.Error(err)
				return
			}

			pkgnet.Respond(w, http.StatusOK, agents)
		})
	return nil
}

func (frontend *Frontend) updateAgentEp(group task.Group, router *mux.Router) error {
	router.Methods("PUT").Path("/v1/agent/{id}").HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
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

			err = frontend.updateAgent(update)
			if err != nil {
				err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
				logger.Error(err)
				return
			}

			pkgnet.RespondEmpty(w, http.StatusOK)
		})
	return nil
}

func (frontend *Frontend) requestSessionEp(group task.Group, router *mux.Router) error {
	router.Methods("POST").Path("/v1/request/session").HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
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
		})
	return nil
}

func (frontend *Frontend) getSessionEp(group task.Group, router *mux.Router) error {
	router.Methods("GET").Path("/v1/session/{id}").HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
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
		})
	return nil
}
