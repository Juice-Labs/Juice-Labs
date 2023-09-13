/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package frontend

import (
	"flag"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/Juice-Labs/Juice-Labs/cmd/controller/storage"
	"github.com/Juice-Labs/Juice-Labs/pkg/errors"
	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
	"github.com/Juice-Labs/Juice-Labs/pkg/server"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
	"github.com/Juice-Labs/Juice-Labs/pkg/utilities"
)

var (
	overrideHostname = flag.String("override-hostname", "", "")
	webhook          = flag.String("webhook-url", "", "")
)

type Frontend struct {
	startTime time.Time

	hostname string

	agentHandlers *utilities.ConcurrentMap[string, *AgentHandler]

	webhookClient   restapi.Client
	webhookMessages chan restapi.WebhookMessage

	storage storage.Storage
}

func NewFrontend(server *server.Server, storage storage.Storage) (*Frontend, error) {
	hostname := *overrideHostname
	if hostname == "" {
		hostname_, err := os.Hostname()
		if err != nil {
			return nil, err
		}

		hostname = hostname_
	}

	frontend := &Frontend{
		startTime:     time.Now(),
		hostname:      hostname,
		agentHandlers: utilities.NewConcurrentMap[string, *AgentHandler](),
		storage:       storage,
	}

	if *webhook != "" {
		frontend.webhookClient = restapi.Client{
			Client:  &http.Client{},
			Address: *webhook,
		}
		frontend.webhookMessages = make(chan restapi.WebhookMessage, 32)
	}

	frontend.initializeEndpoints(server)

	return frontend, nil
}

func (frontend *Frontend) Run(group task.Group) error {
	if frontend.webhookMessages != nil {
		var messages []restapi.WebhookMessage

		messagesCond := sync.NewCond(&sync.Mutex{})

		group.GoFn("Webhook Copying", func(group task.Group) error {
			for {
				select {
				case <-group.Ctx().Done():
					messagesCond.Signal()
					return nil

				case msg := <-frontend.webhookMessages:
					messagesCond.L.Lock()
					messages = append(messages, msg)
					messagesCond.L.Unlock()
					messagesCond.Signal()
				}
			}
		})

		group.GoFn("Webhook Calling", func(g task.Group) error {
			for {
				for len(messages) > 0 {
					select {
					case <-group.Ctx().Done():
						return nil

					default:
						messagesCond.L.Lock()
						msg := messages[0]
						messages = messages[1:]
						messagesCond.L.Unlock()

						body, err := restapi.JsonReaderFromObject(msg)
						if err == nil {
							response, err_ := frontend.webhookClient.PostWithJson(group.Ctx(), *webhook, body)
							if response != nil {
								err = errors.Join(err_, response.Body.Close())
							}
						}

						if err != nil {
							logger.Error(err)
						}
					}
				}

				messagesCond.L.Lock()
				for len(messages) == 0 {
					select {
					case <-group.Ctx().Done():
						return nil

					default:
					}

					messagesCond.Wait()
				}
				messagesCond.L.Unlock()
			}
		})
	}

	return nil
}

func (frontend *Frontend) registerAgent(agent restapi.Agent) (string, error) {
	agent.State = restapi.AgentActive
	return frontend.storage.RegisterAgent(agent)
}

func (frontend *Frontend) getAgents(poolId string) ([]restapi.Agent, error) {
	iterator, err := frontend.storage.GetAgents(poolId)
	if err != nil {
		return nil, err
	}

	agents := make([]restapi.Agent, 0)
	for iterator.Next() {
		agents = append(agents, iterator.Value())
	}

	return agents, nil
}

func (frontend *Frontend) getAgentById(id string) (restapi.Agent, error) {
	return frontend.storage.GetAgentById(id)
}

func (frontend *Frontend) updateAgent(update restapi.AgentUpdate) error {
	err := frontend.storage.UpdateAgent(update)
	if err == nil && len(update.SessionsUpdate) > 0 {
		if frontend.webhookMessages != nil {
			for sessionId, session := range update.SessionsUpdate {
				frontend.webhookMessages <- restapi.WebhookMessage{
					Agent:   update.Id,
					Session: sessionId,
					State:   session.State,
				}
			}
		}
	}

	return err
}

func (frontend *Frontend) requestSession(sessionRequirements restapi.SessionRequirements) (string, error) {
	return frontend.storage.RequestSession(sessionRequirements)
}

func (frontend *Frontend) getSessionById(id string) (restapi.Session, error) {
	return frontend.storage.GetSessionById(id)
}

func (frontend *Frontend) cancelSession(id string) error {
	return frontend.storage.CancelSession(id)
}

func (frontend *Frontend) deletePool(id string) error {
	return frontend.storage.DeletePool(id)
}
func (frontend *Frontend) getPool(id string) (restapi.Pool, error) {
	return frontend.storage.GetPool(id)
}

func (frontend *Frontend) getPoolPermissions(id string) (restapi.PoolPermissions, error) {
	return frontend.storage.GetPoolPermissions(id)
}

func (frontend *Frontend) createPool(name string) (restapi.Pool, error) {
	return frontend.storage.CreatePool(name)
}

func (frontend *Frontend) addPermission(poolId string, userId string, permission restapi.Permission) error {
	return frontend.storage.AddPermission(poolId, userId, permission)
}

func (frontend *Frontend) removePermission(poolId string, userId string, permission restapi.Permission) error {
	return frontend.storage.RemovePermission(poolId, userId, permission)
}

func (frontend *Frontend) getPermissions(userId string) (restapi.UserPermissions, error) {
	return frontend.storage.GetPermissions(userId)
}

func (frontend *Frontend) newAgentHandler(id string) *AgentHandler {
	handler := NewAgentHandler(id)
	frontend.agentHandlers.Set(id, handler)
	return handler
}

func (frontend *Frontend) getAgentHandler(id string) (*AgentHandler, error) {
	handler, found := frontend.agentHandlers.Get(id)
	if !found {
		return nil, errors.Newf("failed to find agent %s", id)
	}

	return handler, nil
}
