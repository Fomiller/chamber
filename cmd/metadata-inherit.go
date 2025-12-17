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
var metadataInheritCmd = &cobra.Command{
	Use:   "inherit <service> <inherited service>",
	Short: "inherit a services secrets for another service",
	Args:  cobra.ExactArgs(1),
	RunE:  inherit,
}

func init() {
	metadataCmd.AddCommand(metadataInheritCmd)
}

func inherit(cmd *cobra.Command, args []string) error {
	service := utils.NormalizeService(args[0])
	if err := validateServiceWithLabel(service); err != nil {
		return fmt.Errorf("Failed to validate service: %w", err)
	}
	// if err := validateServiceWithLabel(inheritedService); err != nil {
	// 	return fmt.Errorf("Failed to validate service: %w", err)
	// }

	if analyticsEnabled && analyticsClient != nil {
		_ = analyticsClient.Enqueue(analytics.Track{
			UserId: username,
			Event:  "Ran Command",
			Properties: analytics.NewProperties().
				Set("command", "inherit").
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

	fmt.Fprint(w, "Service\tInherits")
	fmt.Fprintln(w, "")

	// if withValues {
	// 	fmt.Fprint(w, "\tValue")
	// }
	// fmt.Fprintln(w, "")
	//
	// sort.Sort(ByName(secrets))
	// if sortByTime {
	// 	sort.Sort(ByTime(secrets))
	// }
	// if sortByUser {
	// 	sort.Sort(ByUser(secrets))
	// }
	// if sortByVersion {
	// 	sort.Sort(ByVersion(secrets))
	// }
	//
	visited := make(map[string]bool)
	// printInheritanceTree(cmd.Context(), service, metadataStore, 0, visited)

	// inheritedServices := strings.Join(metadata.Inherits, ", ")
	fmt.Fprintf(w, "%s", key(metadata.Service), buildInheritanceTree(cmd.Context(), service, metadataStore, 0, visited))

	fmt.Fprintln(w, "")

	w.Flush()
	return nil
}

func buildInheritanceTree(ctx context.Context, service string, store store.MetadataStore, depth int, visited map[string]bool) string {
	var b strings.Builder

	// Prevent cycles
	if visited[service] {
		b.WriteString(fmt.Sprintf("%s%s (cycle)\n", strings.Repeat("  ", depth), service))
		return b.String()
	}
	visited[service] = true

	// Print current service
	if depth > 0 {
		b.WriteString(fmt.Sprintf("%s%s\n", strings.Repeat("  ", depth), service))
	}

	// Read inherited services
	metadata, err := store.Read(ctx, service)
	if err != nil {
		b.WriteString(fmt.Sprintf("%s(error: %v)\n", strings.Repeat("  ", depth+1), err))
		return b.String()
	}

	// Recurse into children
	for _, parent := range metadata.Inherits {
		b.WriteString(buildInheritanceTree(ctx, parent, store, depth+1, visited))
	}

	return b.String()
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
