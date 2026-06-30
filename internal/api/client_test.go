package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Fixture header values shared with the SDKs (sdks/INTERFACE.md §5).
var fixtureHeaders = map[string]string{
	"X-Rate-Limit-Limit":     "25000",
	"X-Rate-Limit-Remaining": "24987",
	"X-Rate-Limit-Reset":     "1314000",
}

func newStub(t *testing.T, status int, body string, capture *http.Request) *Client {
	t.Helper()
	c := New("testkey", "demografix-cli/test", 0)
	c.http.Transport = rtFunc(func(r *http.Request) (*http.Response, error) {
		if capture != nil {
			*capture = *r
		}
		h := make(http.Header)
		for k, v := range fixtureHeaders {
			h.Set(k, v)
		}
		return &http.Response{StatusCode: status, Header: h, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	return c
}

func TestGenderizeSingle(t *testing.T) {
	c := newStub(t, 200, `{"name":"peter","gender":"male","probability":1.0,"count":1346866}`, nil)
	preds, quota, err := c.Genderize(context.Background(), []string{"peter"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(preds) != 1 || preds[0].Name != "peter" || preds[0].Gender != "male" || preds[0].Count != 1346866 {
		t.Fatalf("unexpected preds: %+v", preds)
	}
	if quota.Limit != 25000 || quota.Remaining != 24987 || quota.Reset != 1314000 {
		t.Fatalf("unexpected quota: %+v", quota)
	}
}

func TestGenderizeBatchQuery(t *testing.T) {
	var got http.Request
	c := newStub(t, 200, `[{"name":"a","gender":"male","probability":1,"count":1},{"name":"b","gender":"female","probability":1,"count":1}]`, &got)
	c.userAgent = "demografix-cli/1.2.3"

	preds, _, err := c.Genderize(context.Background(), []string{"a", "b"}, "us")
	if err != nil {
		t.Fatal(err)
	}
	if len(preds) != 2 {
		t.Fatalf("want 2 preds, got %d", len(preds))
	}
	if ua := got.Header.Get("User-Agent"); ua != "demografix-cli/1.2.3" {
		t.Errorf("User-Agent = %q", ua)
	}
	q := got.URL.Query()
	if q.Get("apikey") != "testkey" {
		t.Errorf("apikey = %q", q.Get("apikey"))
	}
	if q.Get("country_id") != "us" {
		t.Errorf("country_id = %q", q.Get("country_id"))
	}
	if names := q["name[]"]; len(names) != 2 || names[0] != "a" || names[1] != "b" {
		t.Errorf("name[] = %v", names)
	}
	if q.Get("name") != "" {
		t.Errorf("single name param should be empty for a batch, got %q", q.Get("name"))
	}
}

func TestAgifyNullAge(t *testing.T) {
	c := newStub(t, 200, `{"name":"xyzzy","age":null,"count":0}`, nil)
	preds, _, err := c.Agify(context.Background(), []string{"xyzzy"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if preds[0].Age != nil {
		t.Fatalf("want nil age, got %v", *preds[0].Age)
	}
}

func TestNationalizeShape(t *testing.T) {
	c := newStub(t, 200, `{"name":"nguyen","country":[{"country_id":"VN","probability":0.83},{"country_id":"US","probability":0.04}],"count":29481}`, nil)
	preds, _, err := c.Nationalize(context.Background(), []string{"nguyen"})
	if err != nil {
		t.Fatal(err)
	}
	if len(preds[0].Country) != 2 || preds[0].Country[0].CountryID != "VN" {
		t.Fatalf("unexpected country: %+v", preds[0].Country)
	}
}

func TestErrorMapping(t *testing.T) {
	cases := []struct {
		status int
		body   string
		want   func(error) bool
	}{
		{401, `{"error":"Invalid API key"}`, func(e error) bool { var t *AuthError; return errors.As(e, &t) }},
		{402, `{"error":"Freebie expired"}`, func(e error) bool { var t *SubscriptionError; return errors.As(e, &t) }},
		{422, `{"error":"Invalid 'country_id' parameter"}`, func(e error) bool { var t *ValidationError; return errors.As(e, &t) }},
		{429, `{"error":"Request limit reached"}`, func(e error) bool { var t *RateLimitError; return errors.As(e, &t) }},
	}
	for _, tc := range cases {
		c := newStub(t, tc.status, tc.body, nil)
		_, _, err := c.Genderize(context.Background(), []string{"peter"}, "")
		if err == nil || !tc.want(err) {
			t.Errorf("status %d: unexpected error %v", tc.status, err)
		}
	}
}

func TestRateLimitErrorCarriesQuota(t *testing.T) {
	c := newStub(t, 429, `{"error":"Request limit reached"}`, nil)
	_, _, err := c.Genderize(context.Background(), []string{"peter"}, "")
	var rle *RateLimitError
	if !errors.As(err, &rle) {
		t.Fatalf("want *RateLimitError, got %v", err)
	}
	if rle.Quota == nil || rle.Quota.Reset != 1314000 {
		t.Fatalf("want quota reset 1314000, got %+v", rle.Quota)
	}
}

func TestEmptyKeyFailsBeforeHTTP(t *testing.T) {
	c := New("", "demografix-cli/test", 0)
	c.http.Transport = rtFunc(func(r *http.Request) (*http.Response, error) {
		t.Fatal("HTTP must not be called with an empty key")
		return nil, nil
	})
	_, _, err := c.Genderize(context.Background(), []string{"peter"}, "")
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want *ValidationError, got %v", err)
	}
}

func TestRateLimitEndpoint(t *testing.T) {
	var got http.Request
	c := newStub(t, 200, `{"limit":25000,"remaining":24987,"reset":1314000,"tier":"basic"}`, &got)
	rl, _, err := c.RateLimit(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rl.Tier != "basic" || rl.Limit != 25000 || rl.Remaining != 24987 || rl.Reset != 1314000 {
		t.Fatalf("unexpected rate limit: %+v", rl)
	}
	if !strings.HasSuffix(got.URL.Path, "/rate_limit") {
		t.Errorf("path = %q", got.URL.Path)
	}
}
