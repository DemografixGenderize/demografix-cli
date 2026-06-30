package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/DemografixGenderize/demografix-cli/internal/api"
	"github.com/DemografixGenderize/demografix-cli/internal/config"
	"github.com/DemografixGenderize/demografix-cli/internal/output"
	"github.com/DemografixGenderize/demografix-cli/internal/version"
)

func (a *App) quotaCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "quota",
		Short: "Show the remaining quota for your API key",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := a.client()
			if err != nil {
				return err
			}
			rl, _, err := client.RateLimit(cmd.Context())
			if err != nil {
				return err
			}
			f, err := a.format()
			if err != nil {
				return err
			}
			return output.RenderQuota(a.Out, f, rl, time.Now())
		},
	}
}

func (a *App) loginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Store an API key in the config file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprint(a.Out, "API key: ")
			secret, err := readSecret(a.In)
			fmt.Fprintln(a.Out)
			if err != nil {
				return err
			}
			key := strings.TrimSpace(secret)
			if key == "" {
				return UsageErrorf("no API key entered")
			}
			// Verify before persisting, so a typo'd key surfaces now.
			client := api.New(key, version.UserAgent(), a.timeout)
			if _, _, err := client.RateLimit(cmd.Context()); err != nil {
				return err
			}
			path, err := config.DefaultPath()
			if err != nil {
				return err
			}
			if err := config.Save(path, config.Config{APIKey: key}); err != nil {
				return err
			}
			fmt.Fprintf(a.Out, "Saved to %s\n", prettyPath(path))
			return nil
		},
	}
}

func (a *App) versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the CLI version",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Fprintln(a.Out, version.UserAgent())
		},
	}
}
