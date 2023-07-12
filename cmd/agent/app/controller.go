/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package app

import (
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"time"

	"github.com/Juice-Labs/Juice-Labs/cmd/internal/build"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
)

var (
	controllerAddress    = flag.String("controller", "", "The IP address and port of the controller")
	disableControllerTls = flag.Bool("controller-disable-tls", true, "")
)

type sessionUpdate struct {
	Id    string
	State int
}

type controllerData struct {
	api restapi.Client

	sessionUpdates chan sessionUpdate
}

func (agent *Agent) ConnectToController(group task.Group) error {
	if *controllerAddress != "" {
		agent.api = restapi.Client{
			Client: &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: *disableControllerTls,
					},
				},
			},
			Scheme:  "https",
			Address: *controllerAddress,
		}

		agent.sessionUpdates = make(chan sessionUpdate, agent.maxSessions*4)

		if *disableControllerTls {
			agent.api.Scheme = "http"
		}

		id, err := agent.api.RegisterAgentWithContext(group.Ctx(), restapi.Agent{
			Id:          agent.Id,
			State:       restapi.AgentActive,
			Hostname:    agent.Hostname,
			Address:     agent.Server.Address(),
			Version:     build.Version,
			MaxSessions: agent.maxSessions,
			Gpus:        agent.Gpus.GetGpus(),
			Tags:        agent.tags,
			Taints:      agent.taints,
		})
		if err != nil {
			return fmt.Errorf("Agent.ConnectToController: failed to register with Controller at %s with %s", *controllerAddress, err)
		}

		agent.Id = id

		// When connected to the controller, the agent must not allow requests
		agent.Server.SetCreateEndpoint(RequestSessionName, nil)

		group.GoFn("Controller Update", func(group task.Group) error {
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-group.Ctx().Done():
					return err

				case <-ticker.C:
					// Update our state from what is on the controller
					controllerAgent, err := agent.api.GetAgentWithContext(group.Ctx(), agent.Id)
					if err != nil {
						return err
					}

					for _, session := range controllerAgent.Sessions {
						reference, err_ := agent.getSession(session.Id)

						switch session.State {
						case restapi.SessionAssigned:
							if reference == nil {
								err = errors.Join(err, agent.registerSession(group, session))
							}

						case restapi.SessionCanceling:
							if reference != nil {
								err = errors.Join(err, err_, reference.Object.Cancel())
							}
						}

						if reference != nil {
							reference.Release()
						}
					}

					// Update the controller with our current state
					// Multiple updates can occur within one cycle so create a map to get the latest updates
					sessionsUpdates := map[string]restapi.SessionUpdate{}
					for update := range agent.sessionUpdates {
						sessionsUpdates[update.Id] = restapi.SessionUpdate{
							State: update.State,
						}
					}

					err = errors.Join(err, agent.api.UpdateAgentWithContext(group.Ctx(), restapi.AgentUpdate{
						Id:       agent.Id,
						Sessions: sessionsUpdates,
					}))
					if err != nil {
						return err
					}
				}
			}
		})
	}

	return nil
}

func (agent *Agent) SessionStateChanged(id string, state int) {
	if agent.sessionUpdates != nil {
		agent.sessionUpdates <- sessionUpdate{
			Id:    id,
			State: state,
		}
	}
}
