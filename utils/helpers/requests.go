package helpers

import (
	// Go Internal Packages
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	// Local Packages
	constants "scheduler/utils/constants"
)

var httpClient = &http.Client{Timeout: 3 * time.Minute}

func CallAPI(ctx context.Context, apiURL string, requestType constants.HttpRequestType, body any, headers map[string]string, queryParams map[string]any) (*http.Response, error) {
	// Parse URL and add query parameters
	reqURL, err := url.Parse(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
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
			return nil, fmt.Errorf("unsupported query param type for key %s", key)
		}
	}
	reqURL.RawQuery = q.Encode()

	// Convert body to JSON if not nil
	var reqBody []byte
	if body != nil {
		reqBody, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	// Create the request
	req, err := http.NewRequestWithContext(ctx, requestType.String(), reqURL.String(), bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Send request
	return httpClient.Do(req)
}
