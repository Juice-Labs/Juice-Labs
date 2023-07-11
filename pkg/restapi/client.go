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

type Client struct {
	Client  *http.Client
	Scheme  string
	Address string
}

func (api Client) do(ctx context.Context, method string, path string, contentType string, body io.Reader) (*http.Response, error) {
	url := url.URL{
		Scheme: api.Scheme,
		Host:   api.Address,
		Path:   path,
	}

	request, err := http.NewRequestWithContext(ctx, method, url.String(), body)
	if err != nil {
		return nil, err
	}

	if body != nil {
		request.Header.Add("Content-Type", contentType)
	}

	return api.Client.Do(request)
}

func (api Client) get(ctx context.Context, path string) (*http.Response, error) {
	return api.do(ctx, "GET", path, "", nil)
}

func (api Client) post(ctx context.Context, path string) (*http.Response, error) {
	return api.do(ctx, "POST", path, "", nil)
}

func (api Client) postWithJson(ctx context.Context, path string, body io.Reader) (*http.Response, error) {
	return api.do(ctx, "POST", path, "application/json", body)
}

func (api Client) putWithJson(ctx context.Context, path string, body io.Reader) (*http.Response, error) {
	return api.do(ctx, "PUT", path, "application/json", body)
}

func (api Client) Status() (Status, error) {
	return api.StatusWithContext(context.Background())
}

func (api Client) StatusWithContext(ctx context.Context) (Status, error) {
	response, err := api.get(ctx, "/v1/status")
	if err != nil {
		return Status{}, err
	}
	defer response.Body.Close()

	return parseJsonResponse[Status](response)
}

func (api Client) GetSession(id string) (Session, error) {
	return api.GetSessionWithContext(context.Background(), id)
}

func (api Client) GetSessionWithContext(ctx context.Context, id string) (Session, error) {
	response, err := api.get(ctx, fmt.Sprint("/v1/session", id))
	if err != nil {
		return Session{}, err
	}
	defer response.Body.Close()

	return parseJsonResponse[Session](response)
}

func (api Client) UpdateSession(session Session) error {
	return api.UpdateSessionWithContext(context.Background(), session)
}

func (api Client) UpdateSessionWithContext(ctx context.Context, session Session) error {
	body, err := jsonReaderFromObject(session)
	if err != nil {
		return err
	}

	response, err := api.putWithJson(ctx, fmt.Sprint("/v1/session", session.Id), body)
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
		return "", err
	}

	response, err := api.postWithJson(ctx, "/v1/request/session", body)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	return parseStringResponse(response)
}

func (api Client) ReleaseSession(id string) error {
	return api.ReleaseSessionWithContext(context.Background(), id)
}

func (api Client) ReleaseSessionWithContext(ctx context.Context, id string) error {
	response, err := api.post(ctx, fmt.Sprint("/v1/release/session/", id))
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
	response, err := api.get(ctx, fmt.Sprint("/v1/agent/", id))
	if err != nil {
		return Agent{}, err
	}
	defer response.Body.Close()

	return parseJsonResponse[Agent](response)
}

func (api Client) UpdateAgent(agent Agent) error {
	return api.UpdateAgentWithContext(context.Background(), agent)
}

func (api Client) UpdateAgentWithContext(ctx context.Context, agent Agent) error {
	body, err := jsonReaderFromObject(agent)
	if err != nil {
		return err
	}

	response, err := api.putWithJson(ctx, fmt.Sprint("/v1/agent/", agent.Id), body)
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
		return "", err
	}

	response, err := api.postWithJson(ctx, "/v1/register/agent", body)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	return parseStringResponse(response)
}
