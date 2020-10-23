package ilo4

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"io"
	"net/http"
)

type Client struct {
	Log                 logr.Logger
	Client              *http.Client
	Url                 string
	CredentialsProvider func() (io.Reader, error)
	LoginCounts         prometheus.Counter
}

func (c *Client) GetTemperatures(ctx context.Context) (HealthTemperature, error) {
	return c.doGetTemperatures(ctx, true)
}

func (c *Client) doGetTemperatures(ctx context.Context, retry bool) (HealthTemperature, error) {
	url := c.Url + "/json/health_temperature"
	log := c.Log.WithValues("method", "GET", "url", url)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return HealthTemperature{}, fmt.Errorf("failed to create request: %w", err)
	}
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

	// Handle Forbidden
	if resp.StatusCode == 403 && retry {
		// Login
		if err := c.doLogin(ctx); err != nil {
			return HealthTemperature{}, err
		}
		// Recursive call, without retry
		return c.doGetTemperatures(ctx, false)
	}

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

func (c *Client) doLogin(ctx context.Context) error {
	url := c.Url + "/json/login_session"
	log := c.Log.WithValues("method", "POST", "url", url)

	// Increment counter
	if c.LoginCounts != nil {
		c.LoginCounts.Inc()
	}

	// Get credentials reader
	cred, err := c.CredentialsProvider()
	if err != nil {
		return fmt.Errorf("failed to get credentials reader: %w", err)
	}

	// Close it later, if needed (ignore errors, normally it is closed by http client)
	if credClose, ok := cred.(io.Closer); ok {
		//goland:noinspection GoUnhandledErrorResult
		defer credClose.Close()
	}

	// Prepare request
	req, err := http.NewRequestWithContext(ctx, "POST", url, cred)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Login - it will store cookie in the CookieJar
	log.V(1).Info("sending request")
	resp, err := c.Client.Do(req)
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}

	// Ignore response body
	_ = resp.Body.Close()

	log.Info("got response", "status", resp.Status, "statusCode", resp.StatusCode)

	// Handle other http errors then 2xx
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("login failed with %s", resp.Status)
	}

	// Success
	return nil
}
