/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package app

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/Juice-Labs/Juice-Labs//pkg/api"
	"github.com/Juice-Labs/Juice-Labs//pkg/logger"
	pkgnet "github.com/Juice-Labs/Juice-Labs//pkg/net"
	"github.com/Juice-Labs/Juice-Labs//pkg/utilities"
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
	host   = flag.String("host", "", "The IP address or hostname of the server to connect to")
	port   = flag.Int("port", 0, "The port on the server to connect to")
	pcibus = []string{}
	test   = flag.Bool("test", false, "")

	disableTls = flag.Bool("disable-tls", true, "")

	juicePath = flag.String("juice-path", "", "")
)

func init() {
	flag.Var(&utilities.CommaValue{Value: &pcibus}, "pcibus", "A comma-seperated list of PCI bus addresses as advertised by the server of the form <bus>:<device>.<function> e.g. 01:00.0")
}

func getUrlString(config Configuration, path string) string {
	uri := url.URL{
		Scheme: "https",
		Host:   fmt.Sprintf("%s:%d", config.Host, config.Port),
		Path:   path,
	}

	if *disableTls {
		uri.Scheme = "http"
	}

	return uri.String()
}

func Run(ctx context.Context) error {
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

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: *disableTls,
			},
		},
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

	if *host != "" {
		config.Host = *host
	}
	if config.Host == "" {
		config.Host = "127.0.0.1"
	}

	if *port != 0 {
		config.Port = *port
	}
	if config.Port == 0 {
		config.Port = 43210
	}

	config.PCIBus = pcibus

	config.LogGroup, err = logger.LogLevelAsString()
	if err != nil {
		return err
	}

	var session api.Session

	config.Id, err = pkgnet.PostWithBodyReturnString(client, getUrlString(config, "/v1/request/session"), api.RequestSession{
		Version: "test",
		Gpus:    make([]api.GpuRequirements, 1),
	})
	if err != nil {
		return err
	}

	getSessionUrl := getUrlString(config, fmt.Sprint("/v1/session/", config.Id))
	session, err = pkgnet.Get[api.Session](client, getSessionUrl)
	if err != nil {
		return err
	}

	if session.State == api.StateQueued {
		logger.Info("Session queued")

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for session.State == api.StateQueued {
			select {
			case <-ctx.Done():
				return nil

			case <-ticker.C:
				session, err = pkgnet.Get[api.Session](client, getSessionUrl)
				if err != nil {
					return err
				}
			}
		}

		logger.Info("Session ready")
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
	}

	err = pkgnet.PostWithBodyNoResponse[api.Session](client, getUrlString(config, "/v1/register/session"), session)
	if err != nil {
		return err
	}

	status, err := pkgnet.Get[api.Agent](client, getUrlString(config, "/v1/status"))
	if err != nil {
		return err
	}

	logger.Infof("Connected to %s:%d, v%s", *host, *port, status.Version)

	if *test {
		return nil
	}

	// Make sure we have an application to execute
	if len(flag.Args()) == 0 {
		return errors.New("Usage: juicify [options] <application> <application args>")
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

	err = updateCommand(cmd, config)
	if err == nil {
		err = cmd.Run()
	}

	return nil
}
