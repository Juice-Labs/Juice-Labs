/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package frontend

import (
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/Juice-Labs/Juice-Labs/cmd/internal/build"
	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
	pkgnet "github.com/Juice-Labs/Juice-Labs/pkg/net"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
)

func (frontend *Frontend) initializeEndpoints() {
	frontend.server.AddCreateEndpoint(frontend.getStatusFormer)
	frontend.server.AddCreateEndpoint(frontend.getStatus)
	frontend.server.AddCreateEndpoint(frontend.registerAgent)
	frontend.server.AddCreateEndpoint(frontend.updateAgent)
	frontend.server.AddCreateEndpoint(frontend.requestSession)
	frontend.server.AddCreateEndpoint(frontend.getSession)
}

func (frontend *Frontend) getStatus(router *mux.Router) error {
	router.Methods("GET").Path("/v1/status").HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			err := pkgnet.Respond(w, http.StatusOK, restapi.Status{
				State:    "Active",
				Version:  build.Version,
				Hostname: frontend.hostname,
				Address:  frontend.server.Address(),
			})

			if err != nil {
				err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
				logger.Error(err)
			}
		})
	return nil
}

func (frontend *Frontend) registerAgent(router *mux.Router) error {
	router.Methods("POST").Path("/v1/register/agent").HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			agent, err := pkgnet.ReadRequestBody[restapi.Agent](r)
			if err != nil {
				err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
				logger.Error(err)
				return
			}

			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
				logger.Error(err)
				return
			}

			_, port, err := net.SplitHostPort(agent.Address)
			if err != nil {
				err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
				logger.Error(err)
				return
			}

			agent.Address = fmt.Sprintf("%s:%s", ip, port)
			agent.State = restapi.StateActive

			id, err := frontend.RegisterAgent(agent)
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

func (frontend *Frontend) updateAgent(router *mux.Router) error {
	router.Methods("POST").Path("/v1/agent/{id}").HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			agent, err := pkgnet.ReadRequestBody[restapi.Agent](r)
			if err != nil {
				err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
				logger.Error(err)
				return
			}

			err = frontend.UpdateAgent(agent)
			if err != nil {
				err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
				logger.Error(err)
				return
			}

			pkgnet.RespondEmpty(w, http.StatusOK)
		})
	return nil
}

func (frontend *Frontend) requestSession(router *mux.Router) error {
	router.Methods("POST").Path("/v1/request/session").HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			sessionRequirements, err := pkgnet.ReadRequestBody[restapi.SessionRequirements](r)
			if err != nil {
				err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
				logger.Error(err)
				return
			}

			session, err := frontend.RequestSession(sessionRequirements)
			if err != nil {
				err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
				logger.Error(err)
				return
			}

			err = pkgnet.RespondWithString(w, http.StatusOK, session.Id)
			if err != nil {
				logger.Error(err)
			}
		})
	return nil
}

func (frontend *Frontend) getSession(router *mux.Router) error {
	router.Methods("GET").Path("/v1/session/{id}").HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			id := mux.Vars(r)["id"]

			session, err := frontend.GetSession(id)
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
