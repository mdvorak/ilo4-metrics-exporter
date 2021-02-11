package ilo4

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-logr/logr"
	"net/http"
)

type Credentials struct {
	UserLogin string `json:"user_login"`
	Password  string `json:"password"`
}

type Client struct {
	Log         logr.Logger
	Client      *http.Client
	URL         string
	Credentials Credentials
}

func NewClient(log logr.Logger, httpClient *http.Client, url string, credentials Credentials) *Client {
	return &Client{
		Log:         log,
		Client:      httpClient,
		URL:         url,
		Credentials: credentials,
	}
}

func (c *Client) GetTemperatures(ctx context.Context) (HealthTemperature, error) {
	url := c.URL + "/json/health_temperature"
	log := c.Log.WithValues("method", "GET", "url", url)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return HealthTemperature{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.Credentials.UserLogin, c.Credentials.Password)
	req.Header.Set("Accept", "application/json")

	// Make request
	log.V(1).Info("sending request")
	resp, err := c.Client.Do(req)
	if err != nil {
		return HealthTemperature{}, fmt.Errorf("login failed: %w", err)
	}

	// Close response (ignore errors)
	//goland:noinspection GoUnhandledErrorResult
	defer resp.Body.Close()

	log.Info("got response", "status", resp.Status, "statusCode", resp.StatusCode)

	// Handle other http errors then 2xx
	if resp.StatusCode/100 != 2 {
		return HealthTemperature{}, fmt.Errorf("get temperatures failed with %s", resp.Status)
	}

	// Unmarshal
	result := HealthTemperature{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return HealthTemperature{}, fmt.Errorf("failed to deserialize json response: %w", err)
	}

	// Success
	return result, nil
}
