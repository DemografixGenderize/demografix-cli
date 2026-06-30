package api

// Quota is the remaining-quota view carried on every response via the
// x-rate-limit-* headers. Reset is the number of seconds until the window
// resets.
type Quota struct {
	Limit     int `json:"limit"`
	Remaining int `json:"remaining"`
	Reset     int `json:"reset"`
}

// GenderizePrediction is one genderize.io result. Gender is "male", "female",
// or "" when the API returns null. CountryID is set only when the request was
// scoped to a country.
type GenderizePrediction struct {
	Name        string  `json:"name"`
	Gender      string  `json:"gender"`
	Probability float64 `json:"probability"`
	Count       int     `json:"count"`
	CountryID   string  `json:"country_id,omitempty"`
}

// AgifyPrediction is one agify.io result. Age is nil when the API returns null.
type AgifyPrediction struct {
	Name      string `json:"name"`
	Age       *int   `json:"age"`
	Count     int    `json:"count"`
	CountryID string `json:"country_id,omitempty"`
}

// NationalizeCountry is one candidate country for a nationalize.io result.
type NationalizeCountry struct {
	CountryID   string  `json:"country_id"`
	Probability float64 `json:"probability"`
}

// NationalizePrediction is one nationalize.io result. Country holds up to five
// candidates in descending probability and is empty on no match.
type NationalizePrediction struct {
	Name    string               `json:"name"`
	Country []NationalizeCountry `json:"country"`
	Count   int                  `json:"count"`
}

// RateLimit is the body of the undocumented GET /rate_limit endpoint. The same
// numbers are also carried on the x-rate-limit-* headers.
type RateLimit struct {
	Limit     int    `json:"limit"`
	Remaining int    `json:"remaining"`
	Reset     int    `json:"reset"`
	Tier      string `json:"tier"`
}
