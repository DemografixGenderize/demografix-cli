package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/DemografixGenderize/demografix-cli/internal/api"
	"github.com/DemografixGenderize/demografix-cli/internal/enrich"
	"github.com/DemografixGenderize/demografix-cli/internal/sheet"
)

// addEnrichCmd registers the `enrich` command. The output destination is the
// global -o flag (a file path here, not a format): the output format follows
// the file extension.
func (a *App) addEnrichCmd(root *cobra.Command) {
	var (
		o      enrich.Options
		dryRun bool
	)
	cmd := &cobra.Command{
		Use:   "enrich <file>",
		Short: "Enrich a spreadsheet (CSV/TSV/JSON/JSONL/XLSX) with predictions",
		Long: "Enrich a spreadsheet with gender/age/nationality predictions, joining\n" +
			"results back onto the original rows.\n\n" +
			"For this command the global -o/--output flag is an output FILE PATH, not a\n" +
			"format: the output format follows the file extension (.csv/.tsv/.json/.jsonl/\n" +
			".xlsx). With no -o (or '-') the result is written to stdout in the input format.\n" +
			"Each enabled service appends its full set of columns.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.InputPath = args[0]
			o.OutputPath = a.outputFlag
			o.Concurrency = a.concurrency

			if dryRun {
				info, err := enrich.Plan(o)
				if err != nil {
					return asUsage(err)
				}
				printPlan(a.Out, info)
				return nil
			}

			client, keyErr := a.client()
			if keyErr != nil {
				// Surface flag/column errors before the missing-key error.
				if _, perr := enrich.Plan(o); perr != nil {
					return asUsage(perr)
				}
				return keyErr
			}

			res, runErr := enrich.Run(cmd.Context(), client, o)

			// Configuration/format mistakes are usage errors (exit 2) and never
			// produced output.
			var ce *enrich.ConfigError
			var fe *sheet.UnsupportedFormatError
			if errors.As(runErr, &ce) || errors.As(runErr, &fe) {
				return UsageErrorf("%s", runErr.Error())
			}

			outName := o.OutputPath
			if outName == "" || outName == "-" {
				outName = "stdout"
			}

			// Write whatever was computed, even on a runtime error: batch errors
			// do not abort sibling rows, so the partial output is useful and the
			// user can finish with --resume.
			if res != nil && res.Out != nil {
				if o.OutputPath == "" || o.OutputPath == "-" {
					if err := sheet.WriteTo(a.Out, res.Out); err != nil {
						return err
					}
				} else if err := sheet.WriteFile(o.OutputPath, res.Out); err != nil {
					return err
				}
			}

			if runErr != nil {
				if res != nil {
					var rle *api.RateLimitError
					if errors.As(runErr, &rle) {
						fmt.Fprintf(a.Err, "quota exhausted after %d names; wrote %d of %d rows to %s\n",
							res.NamesBilled, res.RowsWritten, res.RowsTotal, outName)
					} else {
						fmt.Fprintf(a.Err, "stopped on error; wrote %d of %d rows to %s\n",
							res.RowsWritten, res.RowsTotal, outName)
					}
					fmt.Fprintln(a.Err, "retry the remaining rows with --resume")
				}
				return runErr
			}

			fmt.Fprintf(a.Err, "enriched %d rows (%s) → %s\n",
				res.RowsTotal, strings.Join(enrich.ServiceNames(o), ", "), outName)
			a.footer(res.Quota)
			return nil
		},
	}

	f := cmd.Flags()
	f.BoolVar(&o.Gender, "gender", false, "append gender prediction columns")
	f.BoolVar(&o.Age, "age", false, "append age prediction columns")
	f.BoolVar(&o.Nationality, "nationality", false, "append nationality prediction columns")
	f.StringVar(&o.NameCol, "name-col", "", "full-name column (header name or 1-based index)")
	f.StringVar(&o.FirstCol, "first-name-col", "", "first-name column (split mode)")
	f.StringVar(&o.LastCol, "last-name-col", "", "last-name column (split mode)")
	f.StringVar(&o.CountryCol, "country-col", "", "per-row country column (gender/age only)")
	f.StringVar(&o.FixedCountry, "country", "", "fixed ISO country for all rows (gender/age only)")
	f.IntVar(&o.TopN, "top-n", 3, "nationality candidate countries to append (1-5)")
	f.StringVar(&o.Prefix, "prefix", "", "prefix for appended output columns")
	f.BoolVar(&o.Resume, "resume", false, "only fill rows whose prediction columns are empty")
	f.BoolVar(&dryRun, "dry-run", false, "validate and print the cost without calling the API")

	root.AddCommand(cmd)
}

func asUsage(err error) error {
	var ce *enrich.ConfigError
	var fe *sheet.UnsupportedFormatError
	if errors.As(err, &ce) || errors.As(err, &fe) {
		return UsageErrorf("%s", err.Error())
	}
	return err
}

func printPlan(w io.Writer, info *enrich.PlanInfo) {
	out := info.OutputPath
	if out == "" || out == "-" {
		out = "stdout (" + info.OutputFormat + ")"
	}
	fmt.Fprintf(w, "plan\n")
	fmt.Fprintf(w, "  input        %s, %d rows\n", info.InputFormat, info.Rows)
	fmt.Fprintf(w, "  output       %s\n", out)
	fmt.Fprintf(w, "  services     %s\n", strings.Join(info.Services, ", "))
	fmt.Fprintf(w, "  name column  %s\n", info.NameColumns)
	fmt.Fprintf(w, "  country      %s\n", info.Country)
	fmt.Fprintf(w, "  new columns  %s\n", strings.Join(info.OutputColumns, ", "))
	if info.Resume {
		fmt.Fprintf(w, "  cost         %d names (resume) — no API calls made\n", info.Cost)
	} else {
		fmt.Fprintf(w, "  cost         %d names — no API calls made\n", info.Cost)
	}
}
