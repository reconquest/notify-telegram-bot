package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	karma "github.com/reconquest/karma-go"
)

var errorResponse = errors.New("not response from url")

func getJSON(url string) (
	map[string]interface{},
	error,
) {
	var jsonData map[string]interface{}
	var err error

	_, err = http.Get(url)
	if err != nil {
		return nil, errorResponse
	}

	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return jsonData, karma.Format(
			err,
			"unable to get request to url",
		)
	}

	client := &http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		return jsonData, karma.Format(
			err,
			"unable to send an http request",
		)
	}

	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&jsonData)
	if err != nil {
		return jsonData, karma.Format(
			err,
			"unable to decode response body",
		)
	}

	return jsonData, nil
}

func getValueByKey(resource interface{}, keys []string) (interface{}, error) {
	if len(keys) == 0 {
		return resource, nil
	}

	key := keys[0]

	if table, ok := resource.(map[string]interface{}); ok {
		return getValueByKey(table[key], keys[1:])
	} else {
		return nil, fmt.Errorf("expected to see object at field %s", key)
	}
}

func isValidURL(str string) bool {
	isValidURL := strings.Contains(
		str,
		"http",
	)

	if !isValidURL {
		return false
	}

	url, err := url.Parse(str)
	if err != nil {
		return false
	}

	if err == nil && url.Scheme == "" && url.Host == "" {
		return false
	}

	return true
}
