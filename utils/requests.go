package utils

import (
	// Go Internal Packages
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

func CallAPI(apiURL string, requestType HttpRequestType, body interface{}, headers map[string]string, queryParams map[string]interface{}) (*http.Response, error) {
	// Parse URL and add query parameters
	reqURL, err := url.Parse(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %v", err)
	}
	q := reqURL.Query()
	for key, value := range queryParams {
		switch v := value.(type) {
		case string:
			q.Set(key, v)
		case int, int64, float64:
			q.Set(key, fmt.Sprintf("%v", v))
		case bool:
			q.Set(key, strconv.FormatBool(v))
		default:
			return nil, fmt.Errorf("unsupported type for key %s", key)
		}
	}
	reqURL.RawQuery = q.Encode()

	// Convert body to JSON if not nil
	var reqBody []byte
	if body != nil {
		reqBody, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal JSON: %v", err)
		}
	}

	// Create the request
	req, err := http.NewRequest(requestType.String(), reqURL.String(), bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// Add headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Send request
	client := &http.Client{Timeout: 3 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()
	return resp, nil
}
