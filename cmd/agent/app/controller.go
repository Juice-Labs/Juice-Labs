/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package app

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/Juice-Labs/Juice-Labs/cmd/internal/build"
	"github.com/Juice-Labs/Juice-Labs/pkg/errors"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
)

var (
	controllerAddress = flag.String("controller", "", "The IP address and port of the controller")
	accessToken       = flag.String("access-token", "", "The access token to use when connecting to the controller")

	expose = flag.String("expose", "", "The IP address and port to expose through the controller for clients to see. The value is not checked for correctness.")
)

type connectionUpdate struct {
	Id          string
	ExitStatus  string
	Pid         int64
	ProcessName string
}
type sessionUpdate struct {
	Id          string
	State       string
	Connections []connectionUpdate
}

type controllerData struct {
	api restapi.Client

	sessionUpdates chan sessionUpdate

	gpuMetricsMutex sync.Mutex
	gpuMetrics      []restapi.GpuMetrics
}

func (agent *Agent) ConnectToController(group task.Group) error {
	if *controllerAddress != "" {
		accessToken := *accessToken
		if accessToken == "" {
			accessToken = os.Getenv("AUTH0_AGENT_TOKEN")
		}
		agent.api = restapi.Client{
			Client:      &http.Client{},
			Address:     *controllerAddress,
			AccessToken: accessToken,
		}

		// Default queue depth of 32 to limit the amount of potential blocking between updates
		agent.sessionUpdates = make(chan sessionUpdate, 32)

		if *expose == "" {
			return errors.New("--expose must be set when connecting to a controller")
		}

		id, err := agent.api.RegisterAgentWithContext(group.Ctx(), restapi.Agent{
			Id:       agent.Id,
			State:    restapi.AgentActive,
			Hostname: agent.Hostname,
			Address:  *expose,
			Version:  build.Version,
			Gpus:     agent.Gpus.GetGpus(),
			Labels:   agent.labels,
			Taints:   agent.taints,
		})
		if err != nil {
			return fmt.Errorf("Agent.ConnectToController: failed to register with Controller at %s with %s", *controllerAddress, err)
		}

		agent.Id = id

		// When connected to the controller, the agent must not allow requests
		agent.Server.RemoveEndpointByName(RequestSessionName)

		agent.gpuMetrics = make([]restapi.GpuMetrics, agent.Gpus.Count())
		agent.GpuMetricsProvider.AddConsumer(func(gpus []restapi.Gpu) {
			agent.gpuMetricsMutex.Lock()
			defer agent.gpuMetricsMutex.Unlock()

			for index, gpu := range gpus {
				agent.gpuMetrics[index] = gpu.Metrics
			}
		})

		group.GoFn("Controller Update", func(group task.Group) error {
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-group.Ctx().Done():
					return agent.api.UpdateAgent(restapi.AgentUpdate{
						Id:    agent.Id,
						State: restapi.AgentClosed,
					})

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
								err = errors.Join(err, err_, agent.cancelSession(session.Id))
							}
						}

						if reference != nil {
							reference.Release()
						}
					}

					// Update the controller with our current state
					// Multiple updates can occur within one cycle so create a map to get the latest updates
					sessionUpdates := map[string]restapi.SessionUpdate{}

				CopySessions:
					for {
						select {
						case update := <-agent.sessionUpdates:
							connectionUpdates := make([]restapi.Connection, len(update.Connections))
							for index, connection := range update.Connections {
								connectionUpdates[index] = restapi.Connection{
									Id:          connection.Id,
									ExitStatus:  connection.ExitStatus,
									Pid:         connection.Pid,
									ProcessName: connection.ProcessName,
								}
							}

							sessionUpdates[update.Id] = restapi.SessionUpdate{
								State:       update.State,
								Connections: connectionUpdates,
							}

						default:
							break CopySessions
						}
					}

					err = errors.Join(err, agent.api.UpdateAgentWithContext(group.Ctx(), restapi.AgentUpdate{
						Id:             agent.Id,
						State:          restapi.AgentActive,
						SessionsUpdate: sessionUpdates,
						Gpus:           agent.getGpuMetrics(),
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
