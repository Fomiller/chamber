package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// listCmd represents the list command
var metadataInheritsSetCmd = &cobra.Command{
	Use:   "set [service] [inherits]",
	Short: "Set inherited services for a service",
	Long:  "Sets the full list of services this service inherits from (comma-separated).",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		service := args[0]
		raw := args[1]

		inherits := parseCommaList(raw)
		if len(inherits) == 0 {
			return fmt.Errorf("inherits list cannot be empty")
		}

		metadataStore, err := getMetadataStore(cmd.Context())
		if err != nil {
			return fmt.Errorf("Failed to get metadata store: %w", err)
		}
		return metadataStore.SetInherits(cmd.Context(), service, inherits)
	},
}

func init() {
	metadataInheritCmd.AddCommand(metadataInheritsSetCmd)
}

func parseCommaList(input string) []string {
	parts := strings.Split(input, ",")
	out := make([]string, 0, len(parts))

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}

	return out
}

// func key(s string) string {
// 	sep := "/"
//
// 	tokens := strings.Split(s, sep)
// 	secretKey := tokens[len(tokens)-1]
// 	return secretKey
// }
//
// type ByName []store.Secret
//
// func (a ByName) Len() int           { return len(a) }
// func (a ByName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
// func (a ByName) Less(i, j int) bool { return a[i].Meta.Key < a[j].Meta.Key }
//
// type ByTime []store.Secret
//
// func (a ByTime) Len() int           { return len(a) }
// func (a ByTime) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
// func (a ByTime) Less(i, j int) bool { return a[i].Meta.Created.Before(a[j].Meta.Created) }
//
// type ByUser []store.Secret
//
// func (a ByUser) Len() int           { return len(a) }
// func (a ByUser) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
// func (a ByUser) Less(i, j int) bool { return a[i].Meta.CreatedBy < a[j].Meta.CreatedBy }
//
// type ByVersion []store.Secret
//
// func (a ByVersion) Len() int           { return len(a) }
// func (a ByVersion) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
// func (a ByVersion) Less(i, j int) bool { return a[i].Meta.Version < a[j].Meta.Version }
