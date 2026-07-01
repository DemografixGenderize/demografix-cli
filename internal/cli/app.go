// Package cli wires the demografix command tree and owns process exit codes and
// user-facing error rendering.
package cli

import (
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/DemografixGenderize/demografix-cli/internal/api"
	"github.com/DemografixGenderize/demografix-cli/internal/config"
	"github.com/DemografixGenderize/demografix-cli/internal/output"
	"github.com/DemografixGenderize/demografix-cli/internal/version"
)

// App holds global flags and IO. It is the receiver for every command.
type App struct {
	Out io.Writer
	Err io.Writer
	In  *os.File

	outputFlag  string
	timeout     time.Duration
	concurrency int
	noColor     bool
	quiet       bool
}

// Execute runs the CLI and returns a process exit code. SIGINT/SIGTERM cancel
// the command context so a long enrich run can flush partial output.
func Execute() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app := &App{Out: os.Stdout, Err: os.Stderr, In: os.Stdin}
	root := app.rootCmd()
	if err := root.ExecuteContext(ctx); err != nil {
		Render(app.Err, err)
		return Code(err)
	}
	return ExitOK
}

func (a *App) rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "demografix",
		Short:         "Command-line client for the Demografix APIs (genderize, agify, nationalize)",
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	pf := root.PersistentFlags()
	pf.StringVarP(&a.outputFlag, "output", "o", "", "output format: table|json|jsonl|tsv|csv (default: auto)")
	pf.DurationVar(&a.timeout, "timeout", api.DefaultTimeout, "per-request timeout")
	pf.IntVar(&a.concurrency, "concurrency", 4, "maximum concurrent requests")
	pf.BoolVar(&a.noColor, "no-color", false, "disable colored output")
	pf.BoolVarP(&a.quiet, "quiet", "q", false, "suppress the quota footer on stderr")

	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &usageError{msg: err.Error()}
	})
	root.SetOut(a.Out)
	root.SetErr(a.Err)

	root.AddCommand(
		a.genderCmd(),
		a.agifyCmd(),
		a.nationalizeCmd(),
		a.quotaCmd(),
		a.loginCmd(),
		a.versionCmd(),
	)
	a.addEnrichCmd(root)
	return root
}

// client resolves the API key (no keyless fallback) and builds a client.
func (a *App) client() (*api.Client, error) {
	path, err := config.DefaultPath()
	if err != nil {
		return nil, err
	}
	resolved, err := config.ResolveAPIKey(os.Getenv, path)
	if err != nil {
		return nil, err
	}
	return api.New(resolved.Key, version.UserAgent(), a.timeout), nil
}

// format resolves the output format, turning Auto into table/jsonl by TTY.
func (a *App) format() (output.Format, error) {
	f, err := output.ParseFormat(a.outputFlag)
	if err != nil {
		return 0, UsageErrorf("%s", err.Error())
	}
	return output.Resolve(f, isTTY(a.Out)), nil
}

// footer prints the one-line quota summary to stderr unless quiet.
func (a *App) footer(q api.Quota) {
	if a.quiet {
		return
	}
	if line := output.QuotaFooter(q, ""); line != "" {
		_, _ = io.WriteString(a.Err, line+"\n")
	}
}
