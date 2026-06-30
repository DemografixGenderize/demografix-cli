// Package api is the demografix CLI's standalone HTTP client for the three
// Demografix services (genderize.io, agify.io, nationalize.io). It mirrors the
// SDK's wire contract but is independent of it: the User-Agent is injected (so
// CLI traffic is distinguishable in the logs) and an API key is required on
// every request — there is no keyless mode.
package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	// GenderizeBaseURL, AgifyBaseURL and NationalizeBaseURL are the three
	// service hosts. The account is shared across all of them.
	GenderizeBaseURL   = "https://api.genderize.io/"
	AgifyBaseURL       = "https://api.agify.io/"
	NationalizeBaseURL = "https://api.nationalize.io/"

	// DefaultTimeout is the per-request timeout when none is given.
	DefaultTimeout = 10 * time.Second

	// MaxBatch is the largest number of names allowed in one request.
	MaxBatch = 10
)

// Client calls the three services. It is safe for concurrent use.
type Client struct {
	apiKey    string
	userAgent string
	http      *http.Client
}

// New builds a Client. apiKey is required; an empty key makes every call fail
// with a *ValidationError before any HTTP request. userAgent is sent on every
// request. A non-positive timeout falls back to DefaultTimeout.
func New(apiKey, userAgent string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	if userAgent == "" {
		userAgent = "demografix-cli"
	}
	return &Client{
		apiKey:    apiKey,
		userAgent: userAgent,
		http:      &http.Client{Timeout: timeout},
	}
}

// Genderize predicts the gender for up to ten names, returned in input order.
func (c *Client) Genderize(ctx context.Context, names []string, countryID string) ([]GenderizePrediction, Quota, error) {
	return predict[GenderizePrediction](c, ctx, GenderizeBaseURL, names, countryID)
}

// Agify predicts the age for up to ten names, returned in input order.
func (c *Client) Agify(ctx context.Context, names []string, countryID string) ([]AgifyPrediction, Quota, error) {
	return predict[AgifyPrediction](c, ctx, AgifyBaseURL, names, countryID)
}

// Nationalize predicts the nationality for up to ten names, returned in input
// order. The service does not accept a country parameter.
func (c *Client) Nationalize(ctx context.Context, names []string) ([]NationalizePrediction, Quota, error) {
	return predict[NationalizePrediction](c, ctx, NationalizeBaseURL, names, "")
}

// predict bridges the wire shape (an object for one name, an array for a batch)
// to a uniform []T for callers. Methods cannot be generic, so this is a package
// function.
func predict[T any](c *Client, ctx context.Context, base string, names []string, country string) ([]T, Quota, error) {
	switch {
	case len(names) == 0:
		return nil, Quota{}, nil
	case len(names) > MaxBatch:
		return nil, Quota{}, newValidationError("batch holds more than 10 names")
	case len(names) == 1:
		var one T
		q, err := c.do(ctx, base, names, country, &one)
		if err != nil {
			return nil, q, err
		}
		return []T{one}, q, nil
	default:
		var many []T
		q, err := c.do(ctx, base, names, country, &many)
		if err != nil {
			return nil, q, err
		}
		return many, q, nil
	}
}

func (c *Client) do(ctx context.Context, base string, names []string, countryID string, out any) (Quota, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return Quota{}, newValidationError("api key is required")
	}
	req, err := c.buildRequest(ctx, base, names, countryID)
	if err != nil {
		return Quota{}, wrapTransportError(err.Error(), err)
	}
	return c.send(req, out)
}

// buildRequest assembles a GET with name= for a single name or repeated name[]=
// for a batch, plus country_id (when set) and the required apikey.
func (c *Client) buildRequest(ctx context.Context, base string, names []string, countryID string) (*http.Request, error) {
	u, err := url.Parse(base)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	if len(names) == 1 {
		q.Set("name", names[0])
	} else {
		for _, n := range names {
			q.Add("name[]", n)
		}
	}
	if countryID != "" {
		q.Set("country_id", countryID)
	}
	q.Set("apikey", c.apiKey)
	u.RawQuery = q.Encode()
	return c.newGET(ctx, u.String())
}

func (c *Client) newGET(ctx context.Context, rawURL string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")
	return req, nil
}

// send performs the request, parses the quota headers (on success and error),
// and decodes the body into out or maps the status to a typed error.
func (c *Client) send(req *http.Request, out any) (Quota, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		return Quota{}, wrapTransportError(err.Error(), err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Quota{}, wrapTransportError(err.Error(), err)
	}

	quota := parseQuota(resp.Header)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return quota, decodeError(resp.StatusCode, body, quota)
	}
	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return quota, wrapTransportError("response body is not valid JSON: "+err.Error(), err)
		}
	}
	return quota, nil
}

type errorBody struct {
	Error string `json:"error"`
}

func decodeError(status int, body []byte, quota Quota) error {
	var eb errorBody
	if err := json.Unmarshal(body, &eb); err != nil {
		return wrapTransportError("error response body is not valid JSON: "+err.Error(), err)
	}
	q := quota
	return newAPIError(status, eb.Error, &q)
}

func parseQuota(h http.Header) Quota {
	return Quota{
		Limit:     headerInt(h, "X-Rate-Limit-Limit"),
		Remaining: headerInt(h, "X-Rate-Limit-Remaining"),
		Reset:     headerInt(h, "X-Rate-Limit-Reset"),
	}
}

func headerInt(h http.Header, key string) int {
	v := strings.TrimSpace(h.Get(key))
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return n
}
