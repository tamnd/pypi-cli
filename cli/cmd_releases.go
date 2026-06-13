package cli

import (
	"github.com/spf13/cobra"
)

func (a *App) releasesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "releases <name>",
		Short: "List all releases for a package",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			n := a.effectiveLimit(30)
			a.progressf("fetching releases for %q...", args[0])
			releases, err := a.client.Releases(cmd.Context(), args[0], n)
			if err != nil {
				return mapFetchErr(err)
			}
			return a.renderOrEmpty(releases, len(releases))
		},
	}
	return cmd
}
