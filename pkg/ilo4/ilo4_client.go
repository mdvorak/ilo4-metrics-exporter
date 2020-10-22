package ilo4

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"io"
	"io/ioutil"
	"net/http"
)

type Ilo4Client struct {
	Log                 logr.Logger
	Client              *http.Client
	Url                 string
	CredentialsProvider func() (io.Reader, error)
}

func (c *Ilo4Client) DoGetTempratures(ctx context.Context, retry bool) error {
	url := c.Url + "/json/health_temperature"
	log := c.Log.WithValues("method", "GET", "url", url)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	// Make request
	log.V(1).Info("sending request")
	resp, err := c.Client.Do(req)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	// Close response
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.Log.Error(err, "failed to close temperatures response body")
		}
	}()

	log.Info("got response", "status", resp.Status, "statusCode", resp.StatusCode)

	// Handle Forbidden
	if resp.StatusCode == 403 && retry {
		// Login
		if err := c.doLogin(ctx); err != nil {
			return err
		}
		// Recursive call, without retry
		return c.DoGetTempratures(ctx, false)
	}

	// Handle other http errors then 2xx
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("get temperatures failed with %s", resp.Status)
	}

	// Read data
	// TODO
	body, err := ioutil.ReadAll(resp.Body)
	println(resp.Status)
	println(string(body))
	//decoder := json.NewDecoder(resp.Body)

	return nil
}

func (c *Ilo4Client) doLogin(ctx context.Context) error {
	url := c.Url + "/json/login_session"
	log := c.Log.WithValues("method", "POST", "url", url)

	// Get credentials reader
	cred, err := c.CredentialsProvider()
	if err != nil {
		return fmt.Errorf("failed to get credentials reader: %w", err)
	}

	// Close it later, if needed
	if credClose, ok := cred.(io.Closer); ok {
		defer func() {
			if err := credClose.Close(); err != nil {
				c.Log.Error(err, "failed to close credentials file")
			}
		}()
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
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.Log.Error(err, "failed to close login response body")
		}
	}()

	log.Info("got response", "status", resp.Status, "statusCode", resp.StatusCode)

	// Handle other http errors then 2xx
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("login failed with %s", resp.Status)
	}

	// Success
	return nil
}
