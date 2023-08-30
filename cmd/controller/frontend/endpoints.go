/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package frontend

import (
	"errors"
	"fmt"
	"net/http"

	jwtmiddleware "github.com/auth0/go-jwt-middleware/v2"
	"github.com/auth0/go-jwt-middleware/v2/validator"
	"github.com/gorilla/mux"

	"github.com/Juice-Labs/Juice-Labs/cmd/internal/build"
	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
	"github.com/Juice-Labs/Juice-Labs/pkg/middleware"
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
	server.AddEndpointFunc("GET", "/v1/session/{id}", frontend.getSessionEp)
	server.AddEndpointFunc("DELETE", "/v1/session/{id}", frontend.cancelSessionEp)

	server.AddCreateEndpoint(frontend.getAgentsForPoolEp)

	server.AddCreateEndpoint(frontend.createPoolEp)
	server.AddCreateEndpoint(frontend.getPoolEp)
	server.AddCreateEndpoint(frontend.deletePoolEp)

	server.AddCreateEndpoint(frontend.getPermissionsEp)
	server.AddCreateEndpoint(frontend.deletePermissionEp)
	server.AddCreateEndpoint(frontend.addPermissionEp)
}

func (frontend *Frontend) getStatusEp(w http.ResponseWriter, r *http.Request) {
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

func (frontend *Frontend) registerAgentEp(w http.ResponseWriter, r *http.Request) {
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

func (frontend *Frontend) getAgentEp(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	agent, err := frontend.getAgentById(id)
	if err != nil {
		err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
		logger.Error(err)
		return
	}

	pkgnet.Respond(w, http.StatusOK, agent)
}

func (frontend *Frontend) getAgentsEp(w http.ResponseWriter, r *http.Request) {
	agents, err := frontend.getAgents("")
	if err != nil {
		err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
		logger.Error(err)
		return
	}

	pkgnet.Respond(w, http.StatusOK, agents)
}

func (frontend *Frontend) getAgentsForPoolEp(router *mux.Router) error {
	// TODO: Validate claim: create_session:pool_id
	router.Handle("/v1/agents", middleware.EnsureValidToken()(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			poolID := r.URL.Query().Get("pool_id")
			agents, err := frontend.getAgents(poolID)
			if err != nil {
				err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
				logger.Error(err)
				return
			}

			pkgnet.Respond(w, http.StatusOK, agents)
		}))).Methods("GET").Queries("pool_id", "{pool_id}")
	return nil
}

func (frontend *Frontend) updateAgentEp(w http.ResponseWriter, r *http.Request) {
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
}

func (frontend *Frontend) requestSessionEp(w http.ResponseWriter, r *http.Request) {
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

func (frontend *Frontend) cancelSessionEp(w http.ResponseWriter, r *http.Request) {
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

func (frontend *Frontend) getSessionEp(w http.ResponseWriter, r *http.Request) {
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

func (frontend *Frontend) createPoolEp(group task.Group, router *mux.Router) error {
	// Validate org_adming claim?
	router.Handle("/v1/pool", middleware.EnsureValidToken()(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			poolParams, err := pkgnet.ReadRequestBody[restapi.CreatePoolParams](r)
			if err != nil {
				err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
				logger.Error(err)
				return
			}
			pool, err := frontend.createPool(poolParams.Name)
			if err != nil {
				err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
				logger.Error(err)
				return
			}

			// enableValidation := (os.Getenv("ENABLE_TOKEN_VALIDATION") == "true") || *enableTokenValidation
			// TODO: Skip if token validation is disabled
			claims, ok := r.Context().Value(jwtmiddleware.ContextKey{}).(*validator.ValidatedClaims)

			if (!ok) || (claims == nil) {
				err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
				logger.Error(err)
				return
			}

			userId := claims.RegisteredClaims.Subject

			if userId == "" {
				err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
				logger.Error(err)
				return
			}

			err = frontend.addPermission(pool.Id, userId, restapi.PermissionAdmin)
			if err != nil {
				err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
				logger.Error(err)
				return
			}

			err = pkgnet.Respond(w, http.StatusOK, pool)
			if err != nil {
				logger.Error(err)
			}
		}))).Methods("PUT")
	return nil
}

func (frontend *Frontend) getPoolEp(group task.Group, router *mux.Router) error {
	// Validate org_adming claim
	router.Handle("/v1/pool/{id}", middleware.EnsureValidToken()(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			id := mux.Vars(r)["id"]

			pool, err := frontend.getPool(id)
			if err != nil {
				err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
				logger.Error(err)
				return
			}

			err = pkgnet.Respond(w, http.StatusOK, pool)
			if err != nil {
				logger.Error(err)
			}
		}))).Methods("GET")
	return nil
}

func (frontend *Frontend) deletePoolEp(group task.Group, router *mux.Router) error {
	// Validate org_adming claim
	router.Handle("/v1/pool/{id}", middleware.EnsureValidToken()(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			id := mux.Vars(r)["id"]

			err := frontend.deletePool(id)
			if err != nil {
				err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
				logger.Error(err)
				return
			}

			err = pkgnet.Respond(w, http.StatusOK, fmt.Sprintf("Pool %s deleted", id))
			if err != nil {
				logger.Error(err)
			}
		}))).Methods("PUT")
	return nil
}

func (frontend *Frontend) getPermissionsEp(group task.Group, router *mux.Router) error {
	router.Handle("/v1/user/permissions/{id}", middleware.EnsureValidToken()(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			id := mux.Vars(r)["id"]

			permissions, err := frontend.getPermissions(id)
			if err != nil {
				err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
				logger.Error(err)
				return
			}

			err = pkgnet.Respond(w, http.StatusOK, permissions)
			if err != nil {
				logger.Error(err)
			}
		}))).Methods("GET")
	return nil
}

func (frontend *Frontend) deletePermissionEp(group task.Group, router *mux.Router) error {
	router.Handle("/v1/user/permissions", middleware.EnsureValidToken()(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			permissionParams, err := pkgnet.ReadRequestBody[restapi.PermissionParams](r)
			if err != nil {
				err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
				logger.Error(err)
				return
			}

			err = frontend.removePermission(permissionParams.PoolId, permissionParams.UserId, permissionParams.Permission)
			if err != nil {
				err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
				logger.Error(err)
				return
			}

			err = pkgnet.Respond(w, http.StatusOK, fmt.Sprintf("Permission %s deleted", permissionParams.Permission))
			if err != nil {
				logger.Error(err)
			}
		}))).Methods("DELETE")
	return nil
}

func (frontend *Frontend) addPermissionEp(group task.Group, router *mux.Router) error {
	router.Handle("/v1/user/permissions", middleware.EnsureValidToken()(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			permissionParams, err := pkgnet.ReadRequestBody[restapi.PermissionParams](r)
			if err != nil {
				err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
				logger.Error(err)
				return
			}

			err = frontend.addPermission(permissionParams.PoolId, permissionParams.UserId, permissionParams.Permission)
			if err != nil {
				err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
				logger.Error(err)
				return
			}

			err = pkgnet.Respond(w, http.StatusOK, fmt.Sprintf("Permission %s added", permissionParams.Permission))
			if err != nil {
				logger.Error(err)
			}
		}))).Methods("PUT")
	return nil
}
