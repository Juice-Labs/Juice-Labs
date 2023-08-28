/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package app

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Juice-Labs/Juice-Labs/pkg/errors"
	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
)

type Configuration struct {
	Id string `json:"id"`
	Servers []string `json:"servers"`
	AccessToken string `json:"accessToken,omitempty"`
}

var (
	address        = flag.String("address", "", "The IP address or hostname and port of the server to connect to")
	accessToken = flag.String("access-token", "", "The access token to use when connecting to the controller")

	test           = flag.Bool("test", false, "Deprecated: Use --test-connection instead")
	testConnection = flag.Bool("test-connection", false, "Tests the reachability of the controller or server(s)")

	queueTimeout      = flag.Uint("queue-timeout", 0, "Maximum number of seconds to wait for a GPU")
	onQueueTimeout    = flag.String("on-queue-timeout", "fail", "When a queue timeout happens, [fail, continue]")
	onConnectionError = flag.String("on-connection-error", "fail", "When a connection error happens, [fail, continue]")

	juicePath = flag.String("juice-path", "", "Path to the juice executables if different than current executable path")

	errCancelled            = errors.New("cancelled")
	errQueueTimeout         = errors.New("queued GPU request timed out")
	errInvalidConfiguration = errors.New("invalid configuration")
)

func requestSession(group task.Group, client *http.Client, config *Configuration) error {
	api := restapi.Client{
		Client:      client,
		Address:     config.Controller,
		AccessToken: config.AccessToken,
	}

	id, err := api.RequestSessionWithContext(group.Ctx(), config.Requirements)
	if err != nil {
		return err
	}

	session, err := api.GetSessionWithContext(group.Ctx(), id)
	if err != nil {
		return err
	}

	if session.State == restapi.SessionQueued {
		logger.Info("Session queued")

		timeoutChannel := make(chan struct{})
		defer close(timeoutChannel)

		if *queueTimeout > 0 {
			group.GoFn("Queue Timeout", func(g task.Group) error {
				timeout := time.NewTicker(time.Duration(*queueTimeout) * time.Second)
				defer timeout.Stop()

				select {
				case <-group.Ctx().Done():
					return nil

				case <-timeout.C:
					timeoutChannel <- struct{}{}
					return nil
				}
			})
		}

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for session.State == restapi.SessionQueued {
			select {
			case <-group.Ctx().Done():
				err = errCancelled

			case <-timeoutChannel:
				err = errQueueTimeout

			case <-ticker.C:
				session, err = api.GetSessionWithContext(group.Ctx(), id)
			}

			if err != nil {
				logger.Info("Cancelling session")

				api.CancelSession(id)
				return err
			}
		}
	}

	config.Id = session.Id
	config.Servers = []string{session.Address}

	return nil
}

func verifySession(group task.Group, client *http.Client, config Configuration) error {
	// TODO: Verify the connection to the agents

	return nil
}

func Run(group task.Group) error {
	if *test {
		*testConnection = true
	}

	// Make sure we have an application to execute
	if len(flag.Args()) == 0 && !*testConnection {
		return errors.New("usage: juicify [options] <application> [<application args>]")
	}

	switch *onQueueTimeout {
	case "fail":
	case "continue":
		break

	default:
		return errors.Newf("--on-queue-timeout has an invalid valid '%s'", *onQueueTimeout)
	}

	switch *onConnectionError {
	case "fail":
	case "continue":
		break

	default:
		return errors.Newf("--on-queue-timeout has an invalid valid '%s'", *onConnectionError)
	}

	if *juicePath == "" {
		executable, err := os.Executable()
		if err != nil {
			return errors.ErrRuntime.Wrap(err)
		}

		*juicePath = filepath.Dir(executable)
	}

	err := validateHost()
	if err != nil {
		return errors.New("host is not configured properly").Wrap(err)
	}

	cfgPath := filepath.Join(*juicePath, "juice.cfg")

	var config Configuration
	configBytes, err := os.ReadFile(cfgPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return errors.Newf("unable to read config file %s", cfgPath).Wrap(err)
		}
	} else {
		err = json.Unmarshal(configBytes, &config)
		if err != nil {
			return errors.Newf("config file %s has errors", cfgPath).Wrap(err)
		}
	}

	version, err := getVersion()
	if err != nil {
		return errors.New("failed to get client version").Wrap(err)
	}

	config.Requirements.Version = version

	server := *address
	if server != "" {
	// SplitHostPort() rejects addresses that don't have a port or a
	// trailing ":".  Add a trailing ":" to have SplitHostPort() parse
	// the port as 0 and fill in the default port later on to accept
	// hostnames or IP addresses without ports.
		if !strings.Contains(server, ":") {
			server = server + ":"
		}

		config.Servers = []string{server}
	} else if len(config.Servers) > 0 {
		server = config.Servers[0]
	} else {
		return errors.New("require either juice.cfg to have servers set or --address")
	}

	if *accessToken != "" {
		// TODO: Instead of sharing the access token across the controller + client
		// We may want to use seperate audiences (and thus different tokens) for each
		// The controller would generate a token for the client to user using M2M flow with client secret
		config.AccessToken = *accessToken
	}

	client := &http.Client{}

	api := restapi.Client{
		Client:      &http.Client{},
		Address:     server,
		AccessToken: *accessToken,
	}

	status, err := api.StatusWithContext(group.Ctx())
	if err != nil {
		return err
	}

	logger.Infof("Connected to %s, v%s", server, status.Version)

	if *testConnection {
		return nil
	}

	if err == nil {
		err = requestSession(group, client, &config)
		if err != nil {
			if err != errCancelled {
				return err
			}

			return nil
		}
	}

	if err == nil {
		err = verifySession(group, client, config)
	}

	var cmd *exec.Cmd
	if err != nil {
		logger.Error(err.Error())

		if errors.Is(err, errQueueTimeout) {
			if *onQueueTimeout == "fail" {
				return err
			}
		}

		if *onConnectionError == "fail" {
			return err
		}

		logger.Info("Running without Juice")

		args := flag.Args()
		cmd = exec.Command(args[0], args[1:]...)
	} else {
		configOverride, err := json.Marshal(config)
		if err != nil {
			return errors.New("unable to marshal configuration").Wrap(err)
		}

		icdPath := filepath.Join(*juicePath, "JuiceVlk.json")

		cmd = createCommand(flag.Args())
		cmd.Env = append(os.Environ(),
			fmt.Sprintf("VK_ICD_FILENAMES=%s", icdPath),
			fmt.Sprintf("VK_DRIVER_FILES=%s", icdPath),

			fmt.Sprintf("JUICE_CFG_OVERRIDE=%s", string(configOverride)),
		)
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return runCommand(group, cmd, config)
}
