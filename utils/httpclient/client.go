package httpclient

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
)

type Method string

const (
	GET     Method = "GET"
	POST    Method = "POST"
	PATCH   Method = "PATCH"
	DELETE  Method = "DELETE"
	PUT     Method = "PUT"
	HEAD    Method = "HEAD"
	OPTIONS Method = "OPTIONS"
)

func (m Method) Validate() error {
	switch m {
	case GET, POST, PATCH, DELETE, PUT, HEAD, OPTIONS:
		return nil
	default:
		return fmt.Errorf("invalid http method: %s", string(m))
	}
}

func (m Method) String() string {
	return string(m)
}

type Request struct {
	URL         string
	Method      Method
	Headers     map[string]string
	QueryParams map[string]any
	Body        any
}

type Client struct {
	httpClient *http.Client
}

func New() *Client {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		MaxConnsPerHost:     50,
		IdleConnTimeout:     90 * time.Second,
	}
	return &Client{
		httpClient: &http.Client{
			Timeout:   3 * time.Minute,
			Transport: transport,
		},
	}
}

func (c *Client) Do(ctx context.Context, r Request) (*http.Response, error) {
	reqURL, err := url.Parse(r.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	if len(r.QueryParams) > 0 {
		q := reqURL.Query()
		for key, value := range r.QueryParams {
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
	}

	var bodyBytes []byte
	if r.Body != nil {
		bodyBytes, err = json.Marshal(r.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, r.Method.String(), reqURL.String(), bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if r.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range r.Headers {
		req.Header.Set(key, value)
	}

	return c.httpClient.Do(req)
}
