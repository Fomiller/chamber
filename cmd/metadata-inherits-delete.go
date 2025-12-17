package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// listCmd represents the list command
var metadataInheritsDeleteCmd = &cobra.Command{
	Use:   "delete [service] [inherits]",
	Short: "delete inherited services for a service",
	Long:  "Removes services from the list of services this service inherits from (comma-separated).",
	Args:  cobra.ExactArgs(2),
	RunE:  inheritsDelete,
}

func init() {
	metadataInheritsCmd.AddCommand(metadataInheritsDeleteCmd)
}

func inheritsDelete(cmd *cobra.Command, args []string) error {
	service := args[0]
	raw := args[1]

	remove := ParseCommaList(raw)

	metadataStore, err := getMetadataStore(cmd.Context())
	if err != nil {
		return fmt.Errorf("Failed to get metadata store: %w", err)
	}

	metadata, err := metadataStore.Read(cmd.Context(), service)
	if err != nil {
		return fmt.Errorf("Failed to list store contents: %w", err)
	}

	current := map[string]bool{}
	for _, r := range remove {
		current[r] = true
	}

	var updated []string
	for _, v := range metadata.Inherits {
		if !current[v] {
			updated = append(updated, v)
		}
	}

	return metadataStore.SetInherits(cmd.Context(), service, updated)

}
