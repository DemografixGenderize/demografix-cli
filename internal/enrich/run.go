package enrich

import (
	"context"
	"strconv"
	"strings"

	"github.com/DemografixGenderize/demografix-cli/internal/api"
	"github.com/DemografixGenderize/demografix-cli/internal/batch"
	"github.com/DemografixGenderize/demografix-cli/internal/sheet"
)

// Predictor is the subset of the API client the pipeline needs. *api.Client
// satisfies it; tests inject a fake.
type Predictor interface {
	Genderize(ctx context.Context, names []string, country string) ([]api.GenderizePrediction, api.Quota, error)
	Agify(ctx context.Context, names []string, country string) ([]api.AgifyPrediction, api.Quota, error)
	Nationalize(ctx context.Context, names []string) ([]api.NationalizePrediction, api.Quota, error)
}

// Result is the outcome of a run.
type Result struct {
	Out         *sheet.Spreadsheet
	RowsTotal   int
	RowsWritten int
	NamesBilled int
	Quota       api.Quota
}

func prepare(o Options) (*sheet.Spreadsheet, *resolved, error) {
	s, err := sheet.Parse(o.InputPath)
	if err != nil {
		return nil, nil, err
	}
	r, err := resolveConfig(s.Headers, o)
	if err != nil {
		return nil, nil, err
	}
	return s, r, nil
}

// Run enriches the input and returns the built output spreadsheet. On a 429 it
// returns a partial Result plus the *api.RateLimitError so the caller can write
// the partial file and report progress.
func Run(ctx context.Context, p Predictor, o Options) (*Result, error) {
	s, r, err := prepare(o)
	if err != nil {
		return nil, err
	}

	// Resolve the output format before any billable API call, so an unsupported
	// -o extension fails up front instead of after consuming quota.
	outFormat, err := sheet.ResolveOutput(s.Format, o.OutputPath)
	if err != nil {
		return nil, err
	}

	keys := outputKeys(r)
	nRows := len(s.Rows)

	outHeaders := append([]string{}, s.Headers...)
	keyIndex := map[string]int{}
	if r.resume {
		for _, k := range keys {
			if idx := indexOf(s.Headers, k); idx >= 0 {
				keyIndex[k] = idx
			} else {
				keyIndex[k] = len(outHeaders)
				outHeaders = append(outHeaders, k)
			}
		}
	} else {
		// resolveConfig already guaranteed no collision with input headers.
		for _, k := range keys {
			keyIndex[k] = len(outHeaders)
			outHeaders = append(outHeaders, k)
		}
	}

	names := make([]string, nRows)
	countries := make([]string, nRows)
	for i, row := range s.Rows {
		names[i] = combinedName(row, r)
		countries[i] = countryFor(row, r)
	}

	outRows := make([][]sheet.Cell, nRows)
	for i, row := range s.Rows {
		nr := make([]sheet.Cell, len(outHeaders))
		for j := 0; j < len(s.Headers) && j < len(row); j++ {
			nr[j] = row[j]
		}
		outRows[i] = nr
	}

	conc := o.Concurrency
	if conc < 1 {
		conc = 6
	}

	var quota api.Quota
	var runErr error
	billed := 0

	if r.gender && runErr == nil {
		need := needSet(s, r, r.prefix+"gender_count", r.gender)
		res, q, b, err := runOne(ctx, nRows, need, names, countries, conc, synthGender, p.Genderize)
		mergeQuota(&quota, q)
		billed += b
		writeGender(outRows, keyIndex, r.prefix, res)
		runErr = err
	}
	if r.age && runErr == nil {
		need := needSet(s, r, r.prefix+"age_count", r.age)
		res, q, b, err := runOne(ctx, nRows, need, names, countries, conc, synthAge, p.Agify)
		mergeQuota(&quota, q)
		billed += b
		writeAge(outRows, keyIndex, r.prefix, res)
		runErr = err
	}
	if r.nationality && runErr == nil {
		need := needSet(s, r, r.prefix+"nationality_count", r.nationality)
		call := func(c context.Context, ns []string, _ string) ([]api.NationalizePrediction, api.Quota, error) {
			return p.Nationalize(c, ns)
		}
		res, q, b, err := runOne(ctx, nRows, need, names, nil, conc, synthNat, call)
		mergeQuota(&quota, q)
		billed += b
		writeNat(outRows, keyIndex, r.prefix, r.topN, res)
		runErr = err
	}

	out := &sheet.Spreadsheet{
		Format:   outFormat,
		Headers:  outHeaders,
		Rows:     outRows,
		Encoding: sheet.EncUTF8,
		BOM:      sheet.BOMUTF8,
	}
	if outFormat == s.Format {
		out.Meta = s.Meta
	} else {
		out.Meta = sheet.DefaultMeta(outFormat)
	}

	res := &Result{
		Out:         out,
		RowsTotal:   nRows,
		RowsWritten: countWritten(outRows, keyIndex, r),
		NamesBilled: billed,
		Quota:       quota,
	}
	return res, runErr
}

// runOne computes one service over the rows marked in need, returning a pointer
// per row (nil = not computed). Empty-name rows are synthesized locally rather
// than sent to the API.
func runOne[T any](
	ctx context.Context,
	nRows int,
	need []bool,
	names []string,
	countries []string,
	concurrency int,
	synth func(name string) T,
	call func(context.Context, []string, string) ([]T, api.Quota, error),
) ([]*T, api.Quota, int, error) {
	results := make([]*T, nRows)

	var items []batch.Item
	for i := 0; i < nRows; i++ {
		if !need[i] {
			continue
		}
		if strings.TrimSpace(names[i]) == "" {
			v := synth(names[i])
			results[i] = &v
			continue
		}
		country := ""
		if countries != nil {
			country = countries[i]
		}
		items = append(items, batch.Item{Index: i, Name: names[i], Country: country})
	}
	if len(items) == 0 {
		return results, api.Quota{}, 0, nil
	}

	reqs := batch.Plan(items, api.MaxBatch)
	fn := func(c context.Context, rq batch.Request) ([]T, api.Quota, error) {
		return call(c, batch.Names(rq.Items), rq.Country)
	}
	outcomes, quota, err := batch.RunRequests(ctx, reqs, nRows, concurrency, fn)
	billed := 0
	for i := 0; i < nRows; i++ {
		if outcomes[i].Done {
			v := outcomes[i].Value
			results[i] = &v
			billed++
		}
	}
	return results, quota, billed, err
}

func needSet(s *sheet.Spreadsheet, r *resolved, countKey string, enabled bool) []bool {
	need := make([]bool, len(s.Rows))
	if !enabled {
		return need
	}
	if !r.resume {
		for i := range need {
			need[i] = true
		}
		return need
	}
	origIdx := indexOf(s.Headers, countKey)
	if origIdx < 0 {
		for i := range need {
			need[i] = true
		}
		return need
	}
	for i, row := range s.Rows {
		val := ""
		if origIdx < len(row) {
			val = strings.TrimSpace(sheet.CellToString(row[origIdx]))
		}
		need[i] = val == ""
	}
	return need
}

// combinedName mirrors the Elixir extract_name: the full-name cell is used
// verbatim (no trim), and in split mode only parts that are exactly empty are
// dropped before joining with a single space. The same combined string feeds
// every service.
func combinedName(row []sheet.Cell, r *resolved) string {
	if !r.splitMode {
		return cellAt(row, r.fullCol)
	}
	var parts []string
	if r.firstCol >= 0 {
		if v := cellAt(row, r.firstCol); v != "" {
			parts = append(parts, v)
		}
	}
	if r.lastCol >= 0 {
		if v := cellAt(row, r.lastCol); v != "" {
			parts = append(parts, v)
		}
	}
	return strings.Join(parts, " ")
}

func countryFor(row []sheet.Cell, r *resolved) string {
	if r.fixedCountry != "" {
		return r.fixedCountry
	}
	if r.countryCol >= 0 {
		return strings.ToUpper(strings.TrimSpace(cellAt(row, r.countryCol)))
	}
	return ""
}

func cellAt(row []sheet.Cell, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return sheet.CellToString(row[idx])
}

func mergeQuota(dst *api.Quota, q api.Quota) {
	if q.Limit == 0 {
		return
	}
	if dst.Limit == 0 || q.Remaining < dst.Remaining {
		*dst = q
	}
}

// --- synthesized no-match shapes (empty-name rows) -------------------------

func synthGender(name string) api.GenderizePrediction {
	return api.GenderizePrediction{Name: name}
}

func synthAge(name string) api.AgifyPrediction {
	return api.AgifyPrediction{Name: name}
}

func synthNat(name string) api.NationalizePrediction {
	return api.NationalizePrediction{Name: name}
}

// --- write computed cells back ---------------------------------------------

func writeGender(rows [][]sheet.Cell, ki map[string]int, p string, res []*api.GenderizePrediction) {
	for i, g := range res {
		if g == nil {
			continue
		}
		rows[i][ki[p+"gender"]] = nilIfEmpty(g.Gender)
		rows[i][ki[p+"gender_count"]] = int64(g.Count)
		rows[i][ki[p+"gender_probability"]] = g.Probability
	}
}

func writeAge(rows [][]sheet.Cell, ki map[string]int, p string, res []*api.AgifyPrediction) {
	for i, a := range res {
		if a == nil {
			continue
		}
		if a.Age != nil {
			rows[i][ki[p+"age"]] = int64(*a.Age)
		} else {
			rows[i][ki[p+"age"]] = nil
		}
		rows[i][ki[p+"age_count"]] = int64(a.Count)
	}
}

func writeNat(rows [][]sheet.Cell, ki map[string]int, p string, topN int, res []*api.NationalizePrediction) {
	for i, n := range res {
		if n == nil {
			continue
		}
		for k := 1; k <= topN; k++ {
			var id, prob sheet.Cell
			if k-1 < len(n.Country) {
				id = n.Country[k-1].CountryID
				prob = n.Country[k-1].Probability
			}
			rows[i][ki[countryKey(p, k)]] = id
			rows[i][ki[countryProbKey(p, k)]] = prob
		}
		rows[i][ki[p+"nationality_count"]] = int64(n.Count)
	}
}

func countWritten(rows [][]sheet.Cell, ki map[string]int, r *resolved) int {
	var counts []string
	if r.gender {
		counts = append(counts, r.prefix+"gender_count")
	}
	if r.age {
		counts = append(counts, r.prefix+"age_count")
	}
	if r.nationality {
		counts = append(counts, r.prefix+"nationality_count")
	}
	n := 0
	for _, row := range rows {
		done := true
		for _, k := range counts {
			if sheet.CellToString(row[ki[k]]) == "" {
				done = false
				break
			}
		}
		if done {
			n++
		}
	}
	return n
}

func nilIfEmpty(s string) sheet.Cell {
	if s == "" {
		return nil
	}
	return s
}

func countryKey(p string, k int) string     { return p + "country_" + strconv.Itoa(k) }
func countryProbKey(p string, k int) string { return p + "country_" + strconv.Itoa(k) + "_probability" }
