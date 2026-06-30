package api

import (
	"context"
	"net/url"
	"strings"
)

// RateLimit fetches the account quota from the undocumented GET /rate_limit
// endpoint. The account is shared across all three services, so the genderize
// host is used. The JSON body wins; header values fill any field the body omits.
func (c *Client) RateLimit(ctx context.Context) (RateLimit, Quota, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return RateLimit{}, Quota{}, newValidationError("api key is required")
	}

	u, err := url.Parse(GenderizeBaseURL + "rate_limit")
	if err != nil {
		return RateLimit{}, Quota{}, wrapTransportError(err.Error(), err)
	}
	q := url.Values{}
	q.Set("apikey", c.apiKey)
	u.RawQuery = q.Encode()

	req, err := c.newGET(ctx, u.String())
	if err != nil {
		return RateLimit{}, Quota{}, wrapTransportError(err.Error(), err)
	}

	var rl RateLimit
	quota, err := c.send(req, &rl)
	if err != nil {
		return RateLimit{}, quota, err
	}
	if rl.Limit == 0 {
		rl.Limit = quota.Limit
	}
	if rl.Remaining == 0 && rl.Limit != 0 {
		rl.Remaining = quota.Remaining
	}
	if rl.Reset == 0 {
		rl.Reset = quota.Reset
	}
	return rl, quota, nil
}
