package main

import (
	"io/ioutil"
	"net/http"
	"strings"
)

func createClientHTTP(method string, url string) ([]byte, error) {
	var payload *strings.Reader = strings.NewReader("{}")

	client := &http.Client{}

	req, err := http.NewRequest(method, url, payload)

	if err != nil {
		return nil, err
	}

	// req.Header.Add("Accept", "application/json")
	// req.Header.Add("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	return ioutil.ReadAll(res.Body)
}
