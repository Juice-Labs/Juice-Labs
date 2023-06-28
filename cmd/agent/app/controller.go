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
	"net/url"
	"strings"

	"github.com/Juice-Labs/Juice-Labs/internal/build"
	pkgnet "github.com/Juice-Labs/Juice-Labs/pkg/net"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
)

var (
	controllerAddress    = flag.String("controller", "", "The IP address and port of the controller")
	disableControllerTls = flag.Bool("controller-disable-tls", true, "")
	controllerTags       = flag.String("controller-tags", "", "Comma separated list of key=value pairs")
	controllerTaints     = flag.String("controller-taints", "", "Comma separated list of key=value pairs")
)

func getUrlString(path string) string {
	uri := url.URL{
		Scheme: "https",
		Host:   *controllerAddress,
		Path:   path,
	}

	if *disableControllerTls {
		uri.Scheme = "http"
	}

	return uri.String()
}

func (agent *Agent) ConnectToController() error {
	if *controllerAddress != "" {
		var err error

		tags := map[string]string{}
		if *controllerTags != "" {
			for _, tag := range strings.Split(*controllerTags, ",") {
				keyValue := strings.Split(tag, "=")
				if len(keyValue) != 2 {
					err = errors.Join(err, fmt.Errorf("tag '%s' must be in the format key=value", tag))
				} else {
					tags[strings.TrimSpace(keyValue[0])] = strings.TrimSpace(keyValue[1])
				}
			}
		}

		taints := map[string]string{}
		if *controllerTaints != "" {
			for _, taint := range strings.Split(*controllerTaints, ",") {
				keyValue := strings.Split(taint, "=")
				if len(keyValue) != 2 {
					err = errors.Join(err, fmt.Errorf("taint '%s' must be in the format key=value", taint))
				} else {
					taints[strings.TrimSpace(keyValue[0])] = strings.TrimSpace(keyValue[1])
				}
			}
		}

		if err != nil {
			return err
		}

		agent.httpClient = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: *disableControllerTls,
				},
			},
		}

		id, err := pkgnet.PostWithBodyReturnString(agent.httpClient, getUrlString("/v1/register/agent"), restapi.Agent{
			Server: restapi.Server{
				Version:  build.Version,
				Hostname: agent.Hostname,
				Address:  agent.Server.Address(),
			},
			MaxSessions: agent.maxSessions,
			Gpus:        agent.Gpus.GetGpus(),
			Tags:        tags,
			Taints:      taints,
		})
		if err != nil {
			return err
		}

		agent.Server.AddCreateEndpoint(agent.registerSessionEp)

		agent.Id = id
	}

	return nil
}
