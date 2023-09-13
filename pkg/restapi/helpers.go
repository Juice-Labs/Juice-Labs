/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package restapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func parseBody(body io.Reader, length int64) ([]byte, error) {
	if length > 0 {
		message := make([]byte, length)
		n, err := body.Read(message)
		if err != nil && err != io.EOF {
			return nil, err
		} else if length != int64(n) {
			return nil, fmt.Errorf("body length %d did not match expected content length %d", n, length)
		}

		return message, nil
	}

	return nil, nil
}

func parseResponse(response *http.Response, contentType string) ([]byte, error) {
	body, err := parseBody(response.Body, response.ContentLength)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != 200 {
		if body != nil {
			return nil, fmt.Errorf("error received from server, code %d\nmessage: %s", response.StatusCode, string(body))
		}

		return nil, fmt.Errorf("error received from server, code %d", response.StatusCode)
	}

	if !strings.HasPrefix(response.Header.Get("Content-Type"), contentType) {
		return nil, fmt.Errorf("expected Content-Type=%s, received %s", contentType, response.Header.Get("Content-Type"))
	}

	return body, nil
}

func parseJsonResponse[T any](response *http.Response) (T, error) {
	var result T

	body, err := parseResponse(response, "application/json")
	if err != nil {
		return result, err
	}

	err = json.Unmarshal(body, &result)
	return result, err
}

func parseStringResponse(response *http.Response) (string, error) {
	body, err := parseResponse(response, "text/plain")
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func validateResponse(response *http.Response) error {
	body, err := parseBody(response.Body, response.ContentLength)
	if err != nil {
		return err
	}

	if response.StatusCode != 200 {
		if body != nil {
			return fmt.Errorf("error received from server, code %d\nmessage: %s", response.StatusCode, string(body))
		}

		return fmt.Errorf("error received from server, code %d", response.StatusCode)
	}

	return nil
}

func jsonReaderFromObject[T any](object T) (io.Reader, error) {
	data, err := json.Marshal(object)
	if err != nil {
		return nil, err
	}

	return bytes.NewReader(data), nil
}

func JsonReaderFromObject[T any](object T) (io.Reader, error) {
	return jsonReaderFromObject[T](object)
}
