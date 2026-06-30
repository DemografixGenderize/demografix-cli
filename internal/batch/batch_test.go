package batch

import (
	"context"
	"errors"
	"testing"

	"github.com/DemografixGenderize/demografix-cli/internal/api"
)

func TestPlanGroupsAndChunks(t *testing.T) {
	items := []Item{
		{Index: 0, Name: "a", Country: "US"},
		{Index: 1, Name: "b", Country: ""},
		{Index: 2, Name: "c", Country: "US"},
	}
	reqs := Plan(items, 10)
	if len(reqs) != 2 {
		t.Fatalf("want 2 requests, got %d", len(reqs))
	}
	if reqs[0].Country != "" || len(reqs[0].Items) != 1 {
		t.Errorf("req0 = %+v", reqs[0])
	}
	if reqs[1].Country != "US" || len(reqs[1].Items) != 2 {
		t.Errorf("req1 = %+v", reqs[1])
	}
}

func TestChunkSizes(t *testing.T) {
	chunks := Chunk(make([]int, 25), 10)
	if len(chunks) != 3 || len(chunks[0]) != 10 || len(chunks[2]) != 5 {
		t.Fatalf("unexpected chunking: %v", len(chunks))
	}
}

func TestRunRequestsPreservesOrder(t *testing.T) {
	var items []Item
	names := make([]string, 25)
	for i := range names {
		names[i] = "name-" + string(rune('a'+i))
		items = append(items, Item{Index: i, Name: names[i]})
	}
	reqs := Plan(items, 10)
	fn := func(_ context.Context, r Request) ([]string, api.Quota, error) {
		out := make([]string, len(r.Items))
		for i, it := range r.Items {
			out[i] = it.Name
		}
		return out, api.Quota{Limit: 100, Remaining: 90}, nil
	}
	outcomes, quota, err := RunRequests(context.Background(), reqs, len(items), 4, fn)
	if err != nil {
		t.Fatal(err)
	}
	if quota.Limit != 100 {
		t.Errorf("quota = %+v", quota)
	}
	for i := range items {
		if !outcomes[i].Done || outcomes[i].Value != names[i] {
			t.Fatalf("outcome %d = %+v, want %q", i, outcomes[i], names[i])
		}
	}
}

func TestRunRequestsRateLimit(t *testing.T) {
	items := Items([]string{"a", "b"}, "")
	reqs := Plan(items, 10)
	fn := func(_ context.Context, _ Request) ([]string, api.Quota, error) {
		return nil, api.Quota{}, &api.RateLimitError{DemografixError: api.DemografixError{Status: 429, Message: "limit"}}
	}
	_, _, err := RunRequests(context.Background(), reqs, len(items), 2, fn)
	var rle *api.RateLimitError
	if !errors.As(err, &rle) {
		t.Fatalf("want *RateLimitError, got %v", err)
	}
}
