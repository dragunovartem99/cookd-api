package recipes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

func (c *Client) get(ctx context.Context, path string, query url.Values, into any) error {
	if query == nil {
		query = url.Values{}
	}
	query.Set("apiKey", c.key)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path+"?"+query.Encode(), nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		// The URL carries the API key; keep it out of logs.
		return fmt.Errorf("recipe lookup %s: %w", path, urlErrorCause(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("recipe lookup %s: status %d", path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(into)
}

func urlErrorCause(err error) error {
	var e *url.Error
	if errors.As(err, &e) {
		return e.Err
	}
	return err
}
