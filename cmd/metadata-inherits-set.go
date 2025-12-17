package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/segmentio/chamber/v3/store"
	"github.com/spf13/cobra"
)

// listCmd represents the list command
var metadataInheritsSetCmd = &cobra.Command{
	Use:   "set [service] [inherits]",
	Short: "Set inherited services for a service",
	Long:  "Sets the full list of services this service inherits from (comma-separated).",
	Args:  cobra.ExactArgs(2),
	RunE:  inheritsSet,
}

func init() {
	metadataInheritsCmd.AddCommand(metadataInheritsSetCmd)
}

func inheritsSet(cmd *cobra.Command, args []string) error {
	service := args[0]
	raw := args[1]

	inherits := ParseCommaList(raw)

	metadataStore, err := getMetadataStore(cmd.Context())
	if err != nil {
		return fmt.Errorf("Failed to get metadata store: %w", err)
	}

	if err := wouldCreateCycle(cmd.Context(), metadataStore, service, inherits); err != nil {
		return err
	}

	return metadataStore.SetInherits(cmd.Context(), service, inherits)

}

func ParseCommaList(input string) []string {
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

func wouldCreateCycle(
	ctx context.Context,
	store store.MetadataStore,
	service string,
	parents []string,
) error {
	visited := map[string]bool{}

	for _, parent := range parents {
		if parent == service {
			return fmt.Errorf("service %q cannot inherit from itself", service)
		}

		if reachable(ctx, store, parent, service, visited) {
			return fmt.Errorf(
				"cyclic dependency detected: %q already depends on %q",
				parent,
				service,
			)
		}
	}

	return nil
}

func reachable(
	ctx context.Context,
	store store.MetadataStore,
	current string,
	target string,
	visited map[string]bool,
) bool {
	if current == target {
		return true
	}

	if visited[current] {
		return false
	}
	visited[current] = true

	metadata, err := store.Read(ctx, current)
	if err != nil {
		return false
	}

	for _, parent := range metadata.Inherits {
		if reachable(ctx, store, parent, target, visited) {
			return true
		}
	}

	return false
}
