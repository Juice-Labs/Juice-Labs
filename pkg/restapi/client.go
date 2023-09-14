/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package restapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"

	"github.com/gorilla/websocket"

	"github.com/Juice-Labs/Juice-Labs/pkg/errors"
	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
)

var (
	ErrUnableToConnect = errors.New("client: unable to connect")
	ErrInvalidScheme   = errors.New("client: invalid scheme")
	ErrInvalidInput    = errors.New("client: invalid input")
	ErrInvalidResponse = errors.New("client: invalid response")
)

type Client struct {
	Client      *http.Client
	Address     string
	AccessToken string
}

func (api Client) doUrl(ctx context.Context, method string, urlString string, contentType string, body io.Reader) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, method, urlString, body)
	if err != nil {
		return nil, err
	}

	if body != nil {
		request.Header.Add("Content-Type", contentType)
	}

	if api.AccessToken != "" {
		request.Header.Add("Authorization", fmt.Sprintf("Bearer %s", api.AccessToken))
	}

	response, err := api.Client.Do(request)
	if err != nil {
		if opErr, ok := err.(*net.OpError); ok {
			if opErr.Op == "dial" {
				return nil, ErrUnableToConnect.Wrap(err)
			}
		}
		if urlErr, ok := err.(*url.Error); ok {
			if urlErr.Err.Error() == "http: server gave HTTP response to HTTPS client" {
				return nil, ErrInvalidScheme.Wrap(err)
			}
		}
		return nil, err
	}

	return response, nil
}

func (api Client) do(ctx context.Context, method string, path string, contentType string, body io.Reader) (*http.Response, error) {
	pathUrl, err := url.Parse(path)
	if err != nil {
		return nil, err
	}

	pathUrl.Scheme = "https"
	pathUrl.Host = api.Address

	response, err := api.doUrl(ctx, method, pathUrl.String(), contentType, body)

	if err == nil {
		return response, nil
	} else if !errors.Is(err, ErrInvalidScheme) {
		return nil, err
	}

	pathUrl.Scheme = "http"

	return api.doUrl(ctx, method, pathUrl.String(), contentType, body)
}

func (api Client) Get(ctx context.Context, path string) (*http.Response, error) {
	return api.do(ctx, "GET", path, "", nil)
}

func (api Client) Post(ctx context.Context, path string) (*http.Response, error) {
	return api.do(ctx, "POST", path, "", nil)
}

func (api Client) Delete(ctx context.Context, path string) (*http.Response, error) {
	return api.do(ctx, "DELETE", path, "", nil)
}

func (api Client) GetWithJson(ctx context.Context, path string, body io.Reader) (*http.Response, error) {
	return api.do(ctx, "GET", path, "application/json", body)
}

func (api Client) PostWithJson(ctx context.Context, path string, body io.Reader) (*http.Response, error) {
	return api.do(ctx, "POST", path, "application/json", body)
}

func (api Client) PutWithJson(ctx context.Context, path string, body io.Reader) (*http.Response, error) {
	return api.do(ctx, "PUT", path, "application/json", body)
}

func (api Client) Status() (Status, error) {
	return api.StatusWithContext(context.Background())
}

func (api Client) StatusWithContext(ctx context.Context) (Status, error) {
	response, err := api.Get(ctx, "/v1/status")
	if err != nil {
		return Status{}, err
	}
	defer response.Body.Close()

	result, err := parseJsonResponse[Status](response)
	if err != nil {
		return Status{}, ErrInvalidResponse.Wrap(err)
	}

	return result, nil
}

func (api Client) GetSession(id string) (Session, error) {
	return api.GetSessionWithContext(context.Background(), id)
}

func (api Client) GetSessionWithContext(ctx context.Context, id string) (Session, error) {
	response, err := api.Get(ctx, fmt.Sprint("/v1/session/", id))
	if err != nil {
		return Session{}, err
	}
	defer response.Body.Close()

	result, err := parseJsonResponse[Session](response)
	if err != nil {
		return Session{}, ErrInvalidResponse.Wrap(err)
	}

	return result, nil
}

func (api Client) UpdateSession(session Session) error {
	return api.UpdateSessionWithContext(context.Background(), session)
}

func (api Client) UpdateSessionWithContext(ctx context.Context, session Session) error {
	body, err := jsonReaderFromObject(session)
	if err != nil {
		return ErrInvalidInput.Wrap(err)
	}

	response, err := api.PutWithJson(ctx, fmt.Sprint("/v1/session/", session.Id), body)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	return validateResponse(response)
}

func (api Client) RequestSession(requirements SessionRequirements) (string, error) {
	return api.RequestSessionWithContext(context.Background(), requirements)
}

func (api Client) RequestSessionWithContext(ctx context.Context, requirements SessionRequirements) (string, error) {
	body, err := jsonReaderFromObject(requirements)
	if err != nil {
		return "", ErrInvalidInput.Wrap(err)
	}

	response, err := api.PostWithJson(ctx, "/v1/request/session", body)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	return parseStringResponse(response)
}

func (api Client) CancelSession(id string) error {
	return api.CancelSessionWithContext(context.Background(), id)
}

func (api Client) CancelSessionWithContext(ctx context.Context, id string) error {
	response, err := api.Delete(ctx, fmt.Sprint("/v1/session/", id))
	if err != nil {
		return err
	}
	defer response.Body.Close()

	return nil
}

func (api Client) ReleaseSession(id string) error {
	return api.ReleaseSessionWithContext(context.Background(), id)
}

func (api Client) ReleaseSessionWithContext(ctx context.Context, id string) error {
	response, err := api.Post(ctx, fmt.Sprint("/v1/release/session/", id))
	if err != nil {
		return err
	}
	defer response.Body.Close()

	return validateResponse(response)
}

func (api Client) GetAgent(id string) (Agent, error) {
	return api.GetAgentWithContext(context.Background(), id)
}

func (api Client) GetAgentWithContext(ctx context.Context, id string) (Agent, error) {
	response, err := api.Get(ctx, fmt.Sprint("/v1/agent/", id))
	if err != nil {
		return Agent{}, err
	}
	defer response.Body.Close()

	result, err := parseJsonResponse[Agent](response)
	if err != nil {
		return Agent{}, ErrInvalidResponse.Wrap(err)
	}

	return result, nil
}

func (api Client) UpdateAgent(update AgentUpdate) error {
	return api.UpdateAgentWithContext(context.Background(), update)
}

func (api Client) UpdateAgentWithContext(ctx context.Context, update AgentUpdate) error {
	body, err := jsonReaderFromObject(update)
	if err != nil {
		return ErrInvalidInput.Wrap(err)
	}

	response, err := api.PutWithJson(ctx, fmt.Sprint("/v1/agent/", update.Id), body)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	return validateResponse(response)
}

func (api Client) RegisterAgent(agent Agent) (string, error) {
	return api.RegisterAgentWithContext(context.Background(), agent)
}

func (api Client) RegisterAgentWithContext(ctx context.Context, agent Agent) (string, error) {
	body, err := jsonReaderFromObject(agent)
	if err != nil {
		return "", ErrInvalidInput.Wrap(err)
	}

	response, err := api.PostWithJson(ctx, "/v1/register/agent", body)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	return parseStringResponse(response)
}

func (api Client) Connect(ctx context.Context, id string) (string, error) {
	return api.ConnectWithContext(ctx, id)
}

func (api Client) ConnectWithContext(ctx context.Context, id string) (string, error) {
	response, err := api.Post(ctx, fmt.Sprintf("/v1/connect/session/%s", id))
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	logger.Info(response)

	result, err := parseJsonResponse[string](response)
	if err != nil {
		return "", ErrInvalidResponse.Wrap(err)
	}

	return result, nil
}

type MessageResponse struct {
	Topic   string
	Message json.RawMessage
}

type MessageHandler func(msg []byte) (*MessageResponse, error)

func (api Client) doWebsocket(ctx context.Context, path string) (*websocket.Conn, error) {
	pathUrl, err := url.Parse(path)
	if err != nil {
		return nil, err
	}

	pathUrl.Scheme = "ws"
	pathUrl.Host = api.Address

	header := http.Header{}

	if api.AccessToken != "" {
		header.Add("Authorization", fmt.Sprintf("Bearer %s", api.AccessToken))
	}

	ws, _, err := websocket.DefaultDialer.DialContext(ctx, pathUrl.String(), header)
	return ws, err
}

func (api Client) handleWebsocket(ctx context.Context, ws *websocket.Conn, callback MessageHandler) error {
	defer ws.Close()

	wsDone := make(chan error)
	defer close(wsDone)

	go func() {
		for {
			_, msg, err := ws.ReadMessage()
			if err != nil {
				wsDone <- err
				break
			}

			response, err := callback(msg)
			if err != nil {
				wsDone <- err
				break
			}

			if response != nil {
				err = ws.WriteMessage(websocket.TextMessage, response.Message)
				if err != nil {
					wsDone <- err
					break
				}
			}
		}
	}()

	done := false
	for !done {
		select {
		case <-ctx.Done():
			done = true

		case err := <-wsDone:
			return err
		}
	}

	return nil
}

func (api Client) ConnectAgentWithWebsocket(ctx context.Context, id string, callback MessageHandler) error {
	ws, err := api.doWebsocket(ctx, fmt.Sprintf("/v1/agent/%s/connect", id))
	if err != nil {
		return err
	}

	return api.handleWebsocket(ctx, ws, callback)
}
