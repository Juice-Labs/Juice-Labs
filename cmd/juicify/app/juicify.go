/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package app

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
	"github.com/Juice-Labs/Juice-Labs/pkg/utilities"
)

type Configuration struct {
	Id string `json:"id"`

	Host string `json:"host"`

	Port int `json:"port"`

	LogGroup string `json:"logGroup,omitempty"`

	LogFile string `json:"logFile,omitempty"`

	ForceSoftwareDecode bool `json:"forceSoftwareDecode,omitempty"`

	DisableCache bool `json:"disableCache,omitempty"`

	DisableCompression bool `json:"disableCompression,omitempty"`

	Headless bool `json:"headless,omitempty"`

	AllowTearing bool `json:"allowTearing,omitempty"`

	FrameRateLimit int `json:"frameRateLimit,omitempty"`

	FramesQueueAhead int `json:"framesQueueAhead,omitempty"`

	SwapChainBuffers int `json:"swapChainBuffers,omitempty"`

	WaitForDebugger bool `json:"waitForDebugger,omitempty"`

	PCIBus []string `json:"pcibus,omitempty"`

	AccessToken string `json:"accessToken,omitempty"`
}

var (
	address        = flag.String("host", "", "The IP address or hostname and port of the server to connect to")
	test           = flag.Bool("test", false, "Deprecated: Use --test-connection instead")
	testConnection = flag.Bool("test-connection", false, "Verifies juicify is able to reach the server at --address")
	accessToken    = flag.String("access-token", "", "The access token to use when connecting to the server")

	juicePath = flag.String("juice-path", "", "Path to the juice executables if different than current executable path")

	pcibus = []string{}
)

func init() {
	flag.Var(&utilities.CommaValue{Value: &pcibus}, "pcibus", "A comma-seperated list of PCI bus addresses as advertised by the server of the form <bus>:<device>.<function> e.g. 01:00.0")
}

func Run(group task.Group) error {
	if *test {
		*testConnection = true
	}

	// Make sure we have an application to execute
	if len(flag.Args()) == 0 && !*testConnection {
		return errors.New("usage: juicify [options] [<application> <application args>]")
	}

	if *juicePath == "" {
		executable, err := os.Executable()
		if err != nil {
			return err
		}

		*juicePath = filepath.Dir(executable)
	}

	err := validateHost()
	if err != nil {
		return err
	}

	var config Configuration
	configBytes, err := os.ReadFile(filepath.Join(*juicePath, "juice.cfg"))
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	} else {
		err = json.Unmarshal(configBytes, &config)
		if err != nil {
			return err
		}
	}

	if *address != "" {
		// SplitHostPort() rejects addresses that don't have a port or a
		// trailing ":".  Add a trailing ":" to have SplitHostPort() parse
		// the port as 0 and fill in the default port later on to accept
		// hostnames or IP addresses without ports.
		if !strings.Contains(*address, ":") {
			*address = *address + ":"
		}

		host, portStr, err := net.SplitHostPort(*address)
		if err != nil {
			return err
		}

		if portStr != "" {
			port, err := strconv.Atoi(portStr)
			if err != nil {
				return err
			}
			config.Port = port
		}

		config.Host = host
	}

	if config.Host == "" {
		config.Host = "127.0.0.1"
	}

	if config.Port == 0 {
		config.Port = 43210
	}

	config.PCIBus = pcibus

	config.LogGroup, err = logger.LogLevelAsString()
	if err != nil {
		return err
	}

	api := restapi.Client{
		Client:      &http.Client{},
		Address:     fmt.Sprintf("%s:%d", config.Host, config.Port),
		AccessToken: *accessToken,
	}

	if config.Id != "" {
		session, err := api.GetSessionWithContext(group.Ctx(), config.Id)
		if err != nil {
			return err
		}

		if session.State == restapi.SessionQueued {
			logger.Info("Session queued")

			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()

			for session.State == restapi.SessionQueued {
				select {
				case <-group.Ctx().Done():
					return nil

				case <-ticker.C:
					session, err = api.GetSessionWithContext(group.Ctx(), config.Id)
					if err != nil {
						return err
					}
				}
			}
		}

		if session.Address != "" {
			uri := url.URL{
				Host: session.Address,
			}

			hostname := uri.Hostname()
			if hostname != "" {
				config.Host = uri.Hostname()
			}

			portStr := uri.Port()
			if portStr != "" {
				portInt, err := strconv.Atoi(portStr)
				if err != nil {
					return err
				}

				config.Port = portInt
			}

			api.Address = fmt.Sprintf("%s:%d", config.Host, config.Port)
		}
	}

	status, err := api.StatusWithContext(group.Ctx())
	if err != nil {
		return err
	}

	logger.Infof("Connected to %s:%d, v%s", config.Host, config.Port, status.Version)

	if *testConnection {
		return nil
	}

	// TODO: Instead of sharing the access token across the controller + client
	// We may want to use seperate audiences (and thus different tokens) for each
	// The controller would generate a token for the client to user using M2M flow with client secret
	config.AccessToken = *accessToken

	configOverride, err := json.Marshal(config)
	if err != nil {
		return err
	}

	cmd := createCommand(flag.Args())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	icdPath := filepath.Join(*juicePath, "JuiceVlk.json")

	cmd.Env = append(os.Environ(),
		fmt.Sprintf("VK_ICD_FILENAMES=%s", icdPath),
		fmt.Sprintf("VK_DRIVER_FILES=%s", icdPath),

		fmt.Sprintf("JUICE_CFG_OVERRIDE=%s", string(configOverride)),
	)

	return runCommand(group, cmd, config)
}
