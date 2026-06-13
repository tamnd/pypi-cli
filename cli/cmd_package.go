package cli

import (
	"github.com/spf13/cobra"
)

func (a *App) packageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "package <name>",
		Short: "Show package metadata",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a.progressf("fetching package %q...", args[0])
			pkg, err := a.client.Package(cmd.Context(), args[0])
			if err != nil {
				return mapFetchErr(err)
			}
			return a.render(pkg)
		},
	}
	return cmd
}
