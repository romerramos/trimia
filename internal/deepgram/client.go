package deepgram

import (
	"net/http"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.deepgram.com"

type Client struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		APIKey:  strings.TrimSpace(apiKey),
		BaseURL: defaultBaseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Minute,
		},
	}
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}

	return http.DefaultClient
}

func (c *Client) baseURL() string {
	if c.BaseURL != "" {
		return strings.TrimRight(c.BaseURL, "/")
	}

	return defaultBaseURL
}
