/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package frontend

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"

	"github.com/Juice-Labs/Juice-Labs/cmd/internal/build"
	"github.com/Juice-Labs/Juice-Labs/pkg/errors"
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
	server.AddEndpointFunc("GET", "/v1/agent/{id}/connect", frontend.connectAgentEp)
	server.AddEndpointFunc("PUT", "/v1/agent/{id}", frontend.updateAgentEp)
	server.AddEndpointFunc("GET", "/v1/agents", frontend.getAgentsEp)
	server.AddEndpointFunc("POST", "/v1/request/session", frontend.requestSessionEp)
	server.AddEndpointFunc("GET", "/v1/session/{id}", frontend.getSessionEp)
	server.AddEndpointFunc("DELETE", "/v1/session/{id}", frontend.cancelSessionEp)
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
	agents, err := frontend.getAgents()
	if err != nil {
		err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
		logger.Error(err)
		return
	}

	pkgnet.Respond(w, http.StatusOK, agents)
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

func (frontend *Frontend) connectAgentEp(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	_, err := frontend.getAgentById(id)
	if err != nil {
		err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
		logger.Error(err)
		return
	}

	// Agent --> Controller
	if r.Header.Get("Connection") == "Upgrade" && r.Header.Get("Upgrade") == "websocket" {
		ws, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
			logger.Error(err)
			return
		}
		defer ws.Close()

		handler := frontend.newAgentHandler(id)

		wsTask := task.NewTaskManager(r.Context())
		wsTask.GoFn(fmt.Sprintf("Agent %s Read", id), func(group task.Group) error {
			done := false
			for !done {
				_, msg, err := ws.ReadMessage()
				if err != nil {
					return err
				}

				var decodedMessage Message
				err = json.Unmarshal(msg, &decodedMessage)
				if err != nil {
					return err
				}

				err = handler.Publish(decodedMessage.topic, decodedMessage.msg)
				if err != nil {
					return err
				}

				// Check if this agent is done
				select {
				case <-group.Ctx().Done():
					done = true

				default:
					break
				}
			}

			return nil
		})

		wsTask.GoFn(fmt.Sprintf("Agent %s Write", id), func(group task.Group) error {
			msgCh, err := handler.Subscribe(group.Ctx(), "agent")
			if err != nil {
				return err
			}

			done := false
			for !done {
				select {
				case <-group.Ctx().Done():
					done = true

				case msg := <-msgCh:
					err = ws.WriteMessage(websocket.TextMessage, []byte(msg))
					if err != nil {
						return err
					}
				}
			}

			return nil
		})

		// Wait until the agent disconnects
		wsTask.Wait()
	} else {
		// Client --> Controller
		handler, err := frontend.getAgentHandler(id)
		if err != nil {
			err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
			logger.Error(err)
			return
		}

		subscribeCtx, cancelCtx := context.WithCancel(r.Context())

		msgCh, err := handler.Subscribe(subscribeCtx, "rendezvous")
		if err != nil {
			err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
			logger.Error(err)
			return
		}

		msg, err := pkgnet.ReadRequestBodyAsString(r)
		if err != nil {
			err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusBadRequest, err.Error()))
			logger.Error(err)
			return
		}

		err = handler.Publish("agent", []byte(msg))
		if err != nil {
			err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
			logger.Error(err)
			return
		}

		// Wait for the response
		select {
		case <-subscribeCtx.Done():
			break

		case msg := <-msgCh:
			w.WriteHeader(http.StatusOK)
			w.Write(msg)

			cancelCtx()
		}
	}
}
