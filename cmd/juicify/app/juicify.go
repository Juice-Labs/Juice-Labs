/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package app

import (
	"crypto/tls"
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
}

var (
	address        = flag.String("host", "", "The IP address or hostname and port of the server to connect to")
	test           = flag.Bool("test", false, "Deprecated: Use --test-connection instead")
	testConnection = flag.Bool("test-connection", false, "Verifies juicify is able to reach the server at --address")

	disableTls = flag.Bool("disable-tls", true, "Always enabled currently. Disables https when connecting to --address")

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
		host, portStr, err := net.SplitHostPort(*address)
		if err != nil {
			return err
		}

		port, err := strconv.Atoi(portStr)
		if err != nil {
			return err
		}

		config.Host = host
		config.Port = port
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

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: *disableTls,
			},
		},
	}

	api := restapi.Client{
		Client:  client,
		Scheme:  "https",
		Address: fmt.Sprintf("%s:%d", config.Host, config.Port),
	}

	if *disableTls {
		api.Scheme = "http"
	}

	if config.Id != "" {
		session, err := api.GetSessionWithContext(group.Ctx(), config.Id)
		if err != nil {
			return err
		}

		if session.State == restapi.StateQueued {
			logger.Info("Session queued")

			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()

			for session.State == restapi.StateQueued {
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

			config.Host = uri.Hostname()

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
