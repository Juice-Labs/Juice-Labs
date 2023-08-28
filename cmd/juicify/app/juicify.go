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
	Id          string   `json:"id,omitempty"`
	Servers     []string `json:"servers,omitempty"`
	Controller  string   `json:"controller,omitempty"`
	AccessToken string   `json:"accessToken,omitempty"`

	Requirements restapi.SessionRequirements `json:"requirements,omitempty"`
}

var (
	controller  = flag.String("controller", "", "The IP address or hostname and port of the controller")
	accessToken = flag.String("access-token", "", "The access token to use when connecting to the controller")

	test           = flag.Bool("test", false, "Deprecated: Use --test-connection instead")
	testConnection = flag.Bool("test-connection", false, "Tests the reachability of the controller or server(s)")

	juicePath = flag.String("juice-path", "", "Path to the juice executables if different than current executable path")

	errCancelled            = errors.New("cancelled")
	errInvalidConfiguration = errors.New("invalid configuration")
)

func validateConfiguration(config *Configuration) error {
	if config.Id != "" {
		return errInvalidConfiguration.Wrap(errors.New("id must not be specified"))
	} else if len(config.Servers) != 0 {
		return errInvalidConfiguration.Wrap(errors.New("servers must not be specified"))
	} else if config.Controller == "" {
		return errInvalidConfiguration.Wrap(errors.New("controller must be specified"))
	} else if len(config.Requirements.Gpus) == 0 {
		config.Requirements.Gpus = append(config.Requirements.Gpus, restapi.GpuRequirements{})
	}

	return nil
}

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

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for session.State == restapi.SessionQueued {
			select {
			case <-group.Ctx().Done():
				err = errCancelled

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
	config.Controller = ""

	return nil
}

func verifyController(group task.Group, client *http.Client, config Configuration) error {
	api := restapi.Client{
		Client:      client,
		Address:     config.Controller,
		AccessToken: config.AccessToken,
	}

	_, err := api.StatusWithContext(group.Ctx())
	if err != nil {
		return errors.Newf("unable to connect to controller at %s", config.Controller).Wrap(err)
	}

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

	if *controller != "" {
		config.Controller = *controller
	}

	// SplitHostPort() rejects addresses that don't have a port or a
	// trailing ":".  Add a trailing ":" to have SplitHostPort() parse
	// the port as 0 and fill in the default port later on to accept
	// hostnames or IP addresses without ports.
	if !strings.Contains(config.Controller, ":") {
		config.Controller = config.Controller + ":"
	}

	if *accessToken != "" {
		// TODO: Instead of sharing the access token across the controller + client
		// We may want to use seperate audiences (and thus different tokens) for each
		// The controller would generate a token for the client to user using M2M flow with client secret
		config.AccessToken = *accessToken
	}

	err = validateConfiguration(&config)
	if err != nil {
		return err
	}

	client := &http.Client{}

	err = verifyController(group, client, config)
	if *testConnection {
		return err
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
		return err
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
