package cli

import (
	"github.com/spf13/cobra"
)

func (a *App) updatesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "updates",
		Short: "Recent package updates from PyPI RSS",
		RunE: func(cmd *cobra.Command, _ []string) error {
			n := a.effectiveLimit(20)
			a.progressf("fetching recent updates...")
			updates, err := a.client.Updates(cmd.Context(), n)
			if err != nil {
				return mapFetchErr(err)
			}
			return a.renderOrEmpty(updates, len(updates))
		},
	}
	return cmd
}

func (a *App) newestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "newest",
		Short: "Newest packages added to PyPI",
		RunE: func(cmd *cobra.Command, _ []string) error {
			n := a.effectiveLimit(20)
			a.progressf("fetching newest packages...")
			updates, err := a.client.Newest(cmd.Context(), n)
			if err != nil {
				return mapFetchErr(err)
			}
			return a.renderOrEmpty(updates, len(updates))
		},
	}
	return cmd
}
