package cli

import (
	"github.com/spf13/cobra"
)

func (a *App) filesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "files <name> [version]",
		Short: "List distribution files for a release",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			version := ""
			if len(args) == 2 {
				version = args[1]
			}
			a.progressf("fetching files for %q %s...", name, version)
			files, err := a.client.Files(cmd.Context(), name, version)
			if err != nil {
				return mapFetchErr(err)
			}
			return a.renderOrEmpty(files, len(files))
		},
	}
	return cmd
}
