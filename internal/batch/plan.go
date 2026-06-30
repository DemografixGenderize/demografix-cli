// Package batch groups names by country and chunks them to the API's
// ten-per-request cap, then runs the requests with bounded concurrency while
// preserving input order. It is shared by the predict commands and enrich.
package batch

import "sort"

// Item is one name carrying its original input index, which is the key used to
// restore output order regardless of completion order.
type Item struct {
	Index   int
	Name    string
	Country string // normalized; "" means no country (always "" for nationality)
}

// Request is one HTTP call's worth of work: a single country and at most
// MaxBatch names.
type Request struct {
	Country string
	Items   []Item
}

// Items builds positional items for a single fixed country (or "").
func Items(names []string, country string) []Item {
	out := make([]Item, len(names))
	for i, n := range names {
		out[i] = Item{Index: i, Name: n, Country: country}
	}
	return out
}

// Chunk splits xs into slices of at most size elements.
func Chunk[T any](xs []T, size int) [][]T {
	if size <= 0 {
		size = 1
	}
	var out [][]T
	for i := 0; i < len(xs); i += size {
		end := i + size
		if end > len(xs) {
			end = len(xs)
		}
		out = append(out, xs[i:end])
	}
	return out
}

// Plan groups items by country (sorted, for reproducibility) and chunks each
// group to size. Output order is restored later via Item.Index, so request
// order does not affect correctness.
func Plan(items []Item, size int) []Request {
	groups := map[string][]Item{}
	for _, it := range items {
		groups[it.Country] = append(groups[it.Country], it)
	}
	countries := make([]string, 0, len(groups))
	for c := range groups {
		countries = append(countries, c)
	}
	sort.Strings(countries)

	var reqs []Request
	for _, c := range countries {
		for _, chunk := range Chunk(groups[c], size) {
			reqs = append(reqs, Request{Country: c, Items: chunk})
		}
	}
	return reqs
}

// Names returns the names of items in order.
func Names(items []Item) []string {
	ns := make([]string, len(items))
	for i, it := range items {
		ns[i] = it.Name
	}
	return ns
}
