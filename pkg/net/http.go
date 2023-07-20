/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package net

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func Respond[T any](w http.ResponseWriter, code int, obj T) error {
	data, err := json.Marshal(obj)
	if err == nil {
		w.Header().Add("Content-Type", "application/json")
		w.Header().Add("Content-Length", fmt.Sprint(len(data)))
		w.WriteHeader(code)
		_, err = w.Write(data)
	}

	return err
}

func RespondWithString(w http.ResponseWriter, code int, msg string) error {
	w.Header().Add("Content-Type", "text/plain")
	w.Header().Add("Content-Length", fmt.Sprint(len(msg)))
	w.WriteHeader(code)
	_, err := io.WriteString(w, msg)
	return err
}

func RespondEmpty(w http.ResponseWriter, code int) {
	w.WriteHeader(code)
}

func Get[T any](client *http.Client, url string) (T, error) {
	var value T

	r, err := client.Get(url)
	if err != nil {
		return value, err
	}

	defer r.Body.Close()

	return ReadResponseBody[T](r)
}

func Post[T any](client *http.Client, url string) (T, error) {
	var value T

	body := strings.NewReader("")

	r, err := client.Post(url, "text/plain", body)
	if err != nil {
		return value, err
	}

	defer r.Body.Close()

	return ReadResponseBody[T](r)
}

func PostWithBody[T, T1 any](client *http.Client, url string, obj T1) (T, error) {
	var value T

	message, err := json.Marshal(obj)
	if err != nil {
		return value, err
	}

	body := strings.NewReader(string(message))

	r, err := client.Post(url, "application/json", body)
	if err != nil {
		return value, err
	}

	defer r.Body.Close()

	return ReadResponseBody[T](r)
}

func PostWithBodyReturnString[T any](client *http.Client, url string, obj T) (string, error) {
	message, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}

	body := strings.NewReader(string(message))

	r, err := client.Post(url, "application/json", body)
	if err != nil {
		return "", err
	}

	defer r.Body.Close()

	return ReadResponseBodyAsString(r)
}

func PostNoResponse(client *http.Client, url string) error {
	body := strings.NewReader("")

	r, err := client.Post(url, "text/plain", body)
	if err != nil {
		return err
	}

	defer r.Body.Close()

	if r.StatusCode != 200 {
		return fmt.Errorf("error received from server, code %d", r.StatusCode)
	}

	return nil
}

func PostWithBodyNoResponse[T any](client *http.Client, url string, obj T) error {
	message, err := json.Marshal(obj)
	if err != nil {
		return err
	}

	body := strings.NewReader(string(message))

	r, err := client.Post(url, "application/json", body)
	if err != nil {
		return err
	}

	defer r.Body.Close()

	if r.StatusCode != 200 {
		return fmt.Errorf("error received from server, code %d", r.StatusCode)
	}

	return nil
}

func ReadRequestBody[T any](r *http.Request) (T, error) {
	return ReadBody[T](r.Header, http.StatusOK, r.Body, r.ContentLength)
}

func ReadResponseBody[T any](r *http.Response) (T, error) {
	return ReadBody[T](r.Header, r.StatusCode, r.Body, r.ContentLength)
}

func ReadResponseBodyAsString(r *http.Response) (string, error) {
	msg, err := ReadBodyAsBytes(r.Header, r.StatusCode, r.Body, r.ContentLength)
	if err != nil {
		return "", err
	}

	return string(msg), nil
}

func ReadBody[T any](header http.Header, statusCode int, body io.ReadCloser, contentLength int64) (T, error) {
	var value T

	msg, err := ReadBodyAsBytes(header, statusCode, body, contentLength)
	if err != nil {
		return value, err
	}

	if header.Get("Content-Type") != "application/json" {
		return value, fmt.Errorf("expected Content-Type=application/json, received %s", header.Get("Content-Type"))
	}

	err = json.Unmarshal(msg, &value)
	return value, err
}

func ReadBodyAsBytes(header http.Header, statusCode int, body io.ReadCloser, contentLength int64) ([]byte, error) {
	var message []byte
	var err error

	if contentLength > 0 {
		message, err = ParseBody(body, contentLength)
		if err != nil {
			return nil, err
		}
	}

	if statusCode != 200 {
		if message != nil {
			return nil, fmt.Errorf("error received from server, code %d\nmessage: %s", statusCode, string(message))
		}

		return nil, fmt.Errorf("error received from server, code %d", statusCode)
	}

	return message, nil
}

func ParseBody(body io.Reader, length int64) ([]byte, error) {
	message := make([]byte, length)
	n, err := body.Read(message)
	if err != io.EOF {
		return nil, err
	} else if length != int64(n) {
		return nil, fmt.Errorf("body length %d did not match expected content length %d", n, length)
	}

	return message, nil
}
