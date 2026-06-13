package cli

import (
	"github.com/spf13/cobra"
)

func (a *App) depsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deps <name>",
		Short: "List declared dependencies for a package",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a.progressf("fetching deps for %q...", args[0])
			deps, err := a.client.Deps(cmd.Context(), args[0])
			if err != nil {
				return mapFetchErr(err)
			}
			return a.renderOrEmpty(deps, len(deps))
		},
	}
	return cmd
}
