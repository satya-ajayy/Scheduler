package constants

import "fmt"

type HttpRequestType string

const (
	GET     HttpRequestType = "GET"
	POST    HttpRequestType = "POST"
	PATCH   HttpRequestType = "PATCH"
	DELETE  HttpRequestType = "DELETE"
	PUT     HttpRequestType = "PUT"
	HEAD    HttpRequestType = "HEAD"
	OPTIONS HttpRequestType = "OPTIONS"
)

func (t HttpRequestType) Validate() error {
	switch t {
	case GET, POST, PATCH, DELETE, PUT, HEAD, OPTIONS:
		return nil
	default:
		return fmt.Errorf("invalid http request type: %s", string(t))
	}
}

func (t HttpRequestType) String() string {
	return string(t)
}
