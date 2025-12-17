package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	analytics "github.com/segmentio/analytics-go/v3"
	"github.com/segmentio/chamber/v3/store"
	"github.com/segmentio/chamber/v3/utils"
	"github.com/spf13/cobra"
)

// listCmd represents the list command
var metadataInheritsShowCmd = &cobra.Command{
	Use:   "show <service>",
	Short: "show inherited services for a service",
	Long:  "Shows the dependency graph of inherited services in a tree.",
	Args:  cobra.ExactArgs(1),
	RunE:  inheritsShow,
}

func init() {
	metadataInheritsCmd.AddCommand(metadataInheritsShowCmd)
}

func inheritsShow(cmd *cobra.Command, args []string) error {
	service := utils.NormalizeService(args[0])
	if err := validateServiceWithLabel(service); err != nil {
		return fmt.Errorf("Failed to validate service: %w", err)
	}

	if analyticsEnabled && analyticsClient != nil {
		_ = analyticsClient.Enqueue(analytics.Track{
			UserId: username,
			Event:  "Ran Command",
			Properties: analytics.NewProperties().
				Set("command", "inherits").
				Set("chamber-version", chamberVersion).
				Set("service", service).
				Set("backend", backend),
		})
	}

	metadataStore, err := getMetadataStore(cmd.Context())
	if err != nil {
		return fmt.Errorf("Failed to get metadata store: %w", err)
	}

	metadata, err := metadataStore.Read(cmd.Context(), service)
	if err != nil {
		return fmt.Errorf("Failed to list store contents: %w", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, '\t', 0)

	visited := make(map[string]bool)

	fmt.Fprintln(w, "Service")
	fmt.Fprintln(w, service)

	for i, parent := range metadata.Inherits {
		last := i == len(metadata.Inherits)-1
		fmt.Print(buildInheritanceTree(cmd.Context(), parent, metadataStore, "", last, visited))
	}

	fmt.Fprintln(w, "")

	w.Flush()
	return nil
}

func buildInheritanceTree(ctx context.Context, service string, store store.MetadataStore, prefix string, isLast bool, visited map[string]bool) string {
	var b strings.Builder

	// Choose branch symbol
	branch := "├─ "
	nextPrefix := prefix + "│  "
	if isLast {
		branch = "└─ "
		nextPrefix = prefix + "   "
	}

	// Cycle detection
	if visited[service] {
		b.WriteString(fmt.Sprintf("%s%s%s (cycle)\n", prefix, branch, service))
		return b.String()
	}
	visited[service] = true

	// Print this node
	b.WriteString(fmt.Sprintf("%s%s%s\n", prefix, branch, service))

	// Load metadata
	metadata, err := store.Read(ctx, service)
	if err != nil {
		return b.String()
	}

	// Recurse
	for i, parent := range metadata.Inherits {
		last := i == len(metadata.Inherits)-1
		b.WriteString(buildInheritanceTree(ctx, parent, store, nextPrefix, last, visited))
	}

	return b.String()
}
