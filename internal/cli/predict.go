package cli

import (
	"context"
	"io"

	"github.com/spf13/cobra"

	"github.com/DemografixGenderize/demografix-cli/internal/api"
	"github.com/DemografixGenderize/demografix-cli/internal/batch"
	"github.com/DemografixGenderize/demografix-cli/internal/output"
)

// runService groups names by country, chunks them to the ten-per-request cap,
// runs the requests with bounded concurrency, renders the successful results in
// input order, and prints the quota footer. A run error is returned for the
// exit code after partial output is written.
func runService[T any](
	ctx context.Context,
	a *App,
	names []string,
	country string,
	call func(context.Context, batch.Request) ([]T, api.Quota, error),
	render func(io.Writer, output.Format, []T) error,
) error {
	// Resolve the output format before any billable request, so an invalid -o
	// fails as a usage error (exit 2) and never consumes quota.
	f, err := a.format()
	if err != nil {
		return err
	}

	items := batch.Items(names, country)
	reqs := batch.Plan(items, api.MaxBatch)
	outcomes, quota, runErr := batch.RunRequests(ctx, reqs, len(items), a.concurrency, call)

	var preds []T
	for _, o := range outcomes {
		if o.Done {
			preds = append(preds, o.Value)
		}
	}
	if len(preds) == 0 && runErr != nil {
		return runErr
	}

	if err := render(a.Out, f, preds); err != nil {
		return err
	}
	a.footer(quota)
	return runErr
}

func (a *App) genderCmd() *cobra.Command {
	var country string
	cmd := &cobra.Command{
		Use:   "gender [names...]",
		Short: "Predict gender for one or more names (genderize.io)",
		RunE: func(cmd *cobra.Command, args []string) error {
			names, err := a.collectNames(args)
			if err != nil {
				return err
			}
			c := normalizeCountry(country)
			if err := validateCountry(c); err != nil {
				return err
			}
			client, err := a.client()
			if err != nil {
				return err
			}
			call := func(ctx context.Context, r batch.Request) ([]api.GenderizePrediction, api.Quota, error) {
				return client.Genderize(ctx, batch.Names(r.Items), r.Country)
			}
			render := func(w io.Writer, f output.Format, preds []api.GenderizePrediction) error {
				return output.RenderGenderize(w, f, preds, c != "")
			}
			return runService(cmd.Context(), a, names, c, call, render)
		},
	}
	cmd.Flags().StringVarP(&country, "country", "c", "", "ISO 3166-1 alpha-2 country to scope predictions")
	return cmd
}

func (a *App) agifyCmd() *cobra.Command {
	var country string
	cmd := &cobra.Command{
		Use:   "age [names...]",
		Short: "Predict age for one or more names (agify.io)",
		RunE: func(cmd *cobra.Command, args []string) error {
			names, err := a.collectNames(args)
			if err != nil {
				return err
			}
			c := normalizeCountry(country)
			if err := validateCountry(c); err != nil {
				return err
			}
			client, err := a.client()
			if err != nil {
				return err
			}
			call := func(ctx context.Context, r batch.Request) ([]api.AgifyPrediction, api.Quota, error) {
				return client.Agify(ctx, batch.Names(r.Items), r.Country)
			}
			render := func(w io.Writer, f output.Format, preds []api.AgifyPrediction) error {
				return output.RenderAgify(w, f, preds, c != "")
			}
			return runService(cmd.Context(), a, names, c, call, render)
		},
	}
	cmd.Flags().StringVarP(&country, "country", "c", "", "ISO 3166-1 alpha-2 country to scope predictions")
	return cmd
}

func (a *App) nationalizeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "nationality [names...]",
		Short: "Predict nationality for one or more names (nationalize.io)",
		RunE: func(cmd *cobra.Command, args []string) error {
			names, err := a.collectNames(args)
			if err != nil {
				return err
			}
			client, err := a.client()
			if err != nil {
				return err
			}
			call := func(ctx context.Context, r batch.Request) ([]api.NationalizePrediction, api.Quota, error) {
				return client.Nationalize(ctx, batch.Names(r.Items))
			}
			render := func(w io.Writer, f output.Format, preds []api.NationalizePrediction) error {
				return output.RenderNationalize(w, f, preds)
			}
			return runService(cmd.Context(), a, names, "", call, render)
		},
	}
}
