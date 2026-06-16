package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

func (a *App) classifiersCmd() *cobra.Command {
	var filter string

	cmd := &cobra.Command{
		Use:   "classifiers",
		Short: "List all PyPI trove classifiers",
		Long: `Fetch the full list of trove classifiers from PyPI and print them.
Use --filter to show only classifiers whose text contains the given substring.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			a.progressf("fetching classifiers...")
			raw, err := a.client.Classifiers(cmd.Context())
			if err != nil {
				return mapFetchErr(err)
			}

			type classifierRow struct {
				Classifier string `json:"classifier"`
			}
			var rows []classifierRow
			for _, c := range raw {
				if filter != "" && !strings.Contains(strings.ToLower(c), strings.ToLower(filter)) {
					continue
				}
				rows = append(rows, classifierRow{Classifier: c})
			}
			return a.renderOrEmpty(rows, len(rows))
		},
	}

	cmd.Flags().StringVar(&filter, "filter", "", "keep only classifiers containing this substring (case-insensitive)")
	return cmd
}
