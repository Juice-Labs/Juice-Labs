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
	"strings"

	"github.com/Juice-Labs/Juice-Labs/cmd/internal/build"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
)

var (
	controllerAddress    = flag.String("controller", "", "The IP address and port of the controller")
	disableControllerTls = flag.Bool("controller-disable-tls", true, "")
	controllerTags       = flag.String("controller-tags", "", "Comma separated list of key=value pairs")
	controllerTaints     = flag.String("controller-taints", "", "Comma separated list of key=value pairs")
)

func (agent *Agent) ConnectToController(group task.Group) error {
	if *controllerAddress != "" {
		tags := map[string]string{}
		if *controllerTags != "" {
			var err error
			for _, tag := range strings.Split(*controllerTags, ",") {
				keyValue := strings.Split(tag, "=")
				if len(keyValue) != 2 {
					err = errors.Join(err, fmt.Errorf("tag '%s' must be in the format key=value", tag))
				} else {
					tags[strings.TrimSpace(keyValue[0])] = strings.TrimSpace(keyValue[1])
				}
			}

			if err != nil {
				return fmt.Errorf("Agent.ConnectToController: failed to parse --controller-tags with %s", err)
			}
		}

		taints := map[string]string{}
		if *controllerTaints != "" {
			var err error
			for _, taint := range strings.Split(*controllerTaints, ",") {
				keyValue := strings.Split(taint, "=")
				if len(keyValue) != 2 {
					err = errors.Join(err, fmt.Errorf("taint '%s' must be in the format key=value", taint))
				} else {
					taints[strings.TrimSpace(keyValue[0])] = strings.TrimSpace(keyValue[1])
				}
			}

			if err != nil {
				return fmt.Errorf("Agent.ConnectToController: failed to parse --controller-taints with %s", err)
			}
		}

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

		id, err := agent.api.RegisterAgentWithContext(group.Ctx(), restapi.Agent{
			Hostname:    agent.Hostname,
			Address:     agent.Server.Address(),
			Version:     build.Version,
			MaxSessions: agent.maxSessions,
			Gpus:        agent.Gpus.GetGpus(),
			Tags:        tags,
			Taints:      taints,
		})
		if err != nil {
			return fmt.Errorf("Agent.ConnectToController: failed to register with Controller at %s", *controllerAddress)
		}

		agent.Id = id

		// When connected to the controller, the agent must not allow requests
		agent.Server.SetCreateEndpoint(RequestSessionName, nil)
	}

	return nil
}
