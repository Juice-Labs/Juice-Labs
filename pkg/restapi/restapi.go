/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package restapi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type RestApi struct {
	Client  *http.Client
	Scheme  string
	Address string
}

func (api RestApi) do(ctx context.Context, method string, path string, body io.Reader) (*http.Response, error) {
	url := url.URL{
		Scheme: api.Scheme,
		Host:   api.Address,
		Path:   path,
	}

	request, err := http.NewRequestWithContext(ctx, method, url.String(), body)
	if err != nil {
		return nil, err
	}

	return api.Client.Do(request)
}

func (api RestApi) get(ctx context.Context, path string) (*http.Response, error) {
	return api.do(ctx, "GET", path, nil)
}

func (api RestApi) post(ctx context.Context, path string, body io.Reader) (*http.Response, error) {
	return api.do(ctx, "POST", path, body)
}

func (api RestApi) put(ctx context.Context, path string, body io.Reader) (*http.Response, error) {
	return api.do(ctx, "PUT", path, body)
}

func (api RestApi) Status() (Status, error) {
	return api.StatusWithContext(context.Background())
}

func (api RestApi) StatusWithContext(ctx context.Context) (Status, error) {
	response, err := api.get(ctx, "/v1/status")
	if err != nil {
		return Status{}, err
	}
	defer response.Body.Close()

	return parseJsonResponse[Status](response)
}

func (api RestApi) GetSession(id string) (Session, error) {
	return api.GetSessionWithContext(context.Background(), id)
}

func (api RestApi) GetSessionWithContext(ctx context.Context, id string) (Session, error) {
	response, err := api.get(ctx, fmt.Sprint("/v1/session", id))
	if err != nil {
		return Session{}, err
	}
	defer response.Body.Close()

	return parseJsonResponse[Session](response)
}

func (api RestApi) UpdateSession(session Session) error {
	return api.UpdateSessionWithContext(context.Background(), session)
}

func (api RestApi) UpdateSessionWithContext(ctx context.Context, session Session) error {
	body, err := jsonReaderFromObject(session)
	if err != nil {
		return err
	}

	response, err := api.put(ctx, fmt.Sprint("/v1/session", session.Id), body)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	return validateResponse(response)
}

func (api RestApi) RequestSession(requirements SessionRequirements) (string, error) {
	return api.RequestSessionWithContext(context.Background(), requirements)
}

func (api RestApi) RequestSessionWithContext(ctx context.Context, requirements SessionRequirements) (string, error) {
	body, err := jsonReaderFromObject(requirements)
	if err != nil {
		return "", err
	}

	response, err := api.post(ctx, "/v1/request/session", body)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	return parseStringResponse(response)
}

func (api RestApi) ReleaseSession(id string) error {
	return api.ReleaseSessionWithContext(context.Background(), id)
}

func (api RestApi) ReleaseSessionWithContext(ctx context.Context, id string) error {
	response, err := api.post(ctx, fmt.Sprint("/v1/release/session/", id), nil)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	return validateResponse(response)
}

func (api RestApi) GetAgent(id string) (Agent, error) {
	return api.GetAgentWithContext(context.Background(), id)
}

func (api RestApi) GetAgentWithContext(ctx context.Context, id string) (Agent, error) {
	response, err := api.get(ctx, fmt.Sprint("/v1/agent", id))
	if err != nil {
		return Agent{}, err
	}
	defer response.Body.Close()

	return parseJsonResponse[Agent](response)
}

func (api RestApi) UpdateAgent(agent Agent) error {
	return api.UpdateAgentWithContext(context.Background(), agent)
}

func (api RestApi) UpdateAgentWithContext(ctx context.Context, agent Agent) error {
	body, err := jsonReaderFromObject(agent)
	if err != nil {
		return err
	}

	response, err := api.put(ctx, fmt.Sprint("/v1/agent", agent.Id), body)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	return validateResponse(response)
}

func (api RestApi) RegisterAgent(agent Agent) (string, error) {
	return api.RegisterAgentWithContext(context.Background(), agent)
}

func (api RestApi) RegisterAgentWithContext(ctx context.Context, agent Agent) (string, error) {
	body, err := jsonReaderFromObject(agent)
	if err != nil {
		return "", err
	}

	response, err := api.post(ctx, "/v1/register/agent", body)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	return parseStringResponse(response)
}
