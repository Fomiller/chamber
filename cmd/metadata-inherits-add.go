package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// listCmd represents the list command
var metadataInheritsAddCmd = &cobra.Command{
	Use:   "add [service] [inherits]",
	Short: "add an inherited services for a service",
	Long:  "Removes services from the list of services this service inherits from (comma-separated).",
	Args:  cobra.ExactArgs(2),
	RunE:  inheritsAdd,
}

func init() {
	metadataInheritsCmd.AddCommand(metadataInheritsAddCmd)
}

func inheritsAdd(cmd *cobra.Command, args []string) error {
	service := args[0]
	raw := args[1]

	add := ParseCommaList(raw)

	metadataStore, err := getMetadataStore(cmd.Context())
	if err != nil {
		return fmt.Errorf("Failed to get metadata store: %w", err)
	}

	if err := wouldCreateCycle(cmd.Context(), metadataStore, service, add); err != nil {
		return err
	}

	return metadataStore.AddInherits(cmd.Context(), service, add)
}
