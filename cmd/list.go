package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	analytics "github.com/segmentio/analytics-go/v3"
	"github.com/segmentio/chamber/v3/store"
	"github.com/segmentio/chamber/v3/utils"
	"github.com/spf13/cobra"
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list <service>",
	Short: "List the secrets set for a service",
	Args:  cobra.ExactArgs(1),
	RunE:  list,
}

var (
	withValues    bool
	sortByTime    bool
	sortByUser    bool
	sortByVersion bool
)

func init() {
	listCmd.Flags().BoolVarP(&withValues, "expand", "e", false, "Expand parameter list with values")
	listCmd.Flags().BoolVarP(&sortByTime, "time", "t", false, "Sort by modified time")
	listCmd.Flags().BoolVarP(&sortByUser, "user", "u", false, "Sort by user")
	listCmd.Flags().BoolVarP(&sortByVersion, "version", "v", false, "Sort by version")
	RootCmd.AddCommand(listCmd)
}

func list(cmd *cobra.Command, args []string) error {
	service := utils.NormalizeService(args[0])
	if err := validateServiceWithLabel(service); err != nil {
		return fmt.Errorf("Failed to validate service: %w", err)
	}

	if analyticsEnabled && analyticsClient != nil {
		_ = analyticsClient.Enqueue(analytics.Track{
			UserId: username,
			Event:  "Ran Command",
			Properties: analytics.NewProperties().
				Set("command", "list").
				Set("chamber-version", chamberVersion).
				Set("service", service).
				Set("backend", backend),
		})
	}

	secretStore, err := getSecretStore(cmd.Context())
	if err != nil {
		return fmt.Errorf("Failed to get secret store: %w", err)
	}

	// metadataStore, err := getMetadataStore(cmd.Context())
	// if err != nil {
	// 	return fmt.Errorf("Failed to get secret store: %w", err)
	// }
	// metadata, err := metadataStore.Read(cmd.Context(), service)
	// if err != nil {
	// 	return fmt.Errorf("Failed to list store contents: %w", err)
	// }

	// services := append([]string{service}, metadata.Inherits...)

	// var secrets []store.Secret
	// for _, service := range services {
	// 	_secrets, err := secretStore.List(cmd.Context(), service, withValues)
	// 	if err != nil {
	// 		return fmt.Errorf("Failed to list store contents: %w", err)
	// 	}
	// 	secrets = append(secrets, _secrets...)
	// }
	// Usage:
	visited := make(map[string]bool)
	merged := make(map[string]store.Secret)
	err = collectSecrets(cmd.Context(), service, secretStore, visited, merged)
	if err != nil {
		return err
	}

	// Convert to slice
	secrets := make([]store.Secret, 0, len(merged))
	for _, s := range merged {
		secrets = append(secrets, s)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, '\t', 0)

	fmt.Fprint(w, "Key\tVersion\tLastModified\tService\tUser")
	if withValues {
		fmt.Fprint(w, "\tValue")
	}
	fmt.Fprintln(w, "")

	sort.Sort(ByName(secrets))
	if sortByTime {
		sort.Sort(ByTime(secrets))
	}
	if sortByUser {
		sort.Sort(ByUser(secrets))
	}
	if sortByVersion {
		sort.Sort(ByVersion(secrets))
	}

	for _, secret := range secrets {
		fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%s",
			key(secret.Meta.Key),
			secret.Meta.Version,
			secret.Meta.Created.Local().Format(ShortTimeFormat),
			serviceName(secret.Meta.Key),
			secret.Meta.CreatedBy)

		if withValues {
			fmt.Fprintf(w, "\t%s", *secret.Value)
		}
		fmt.Fprintln(w, "")
	}

	w.Flush()
	return nil
}

// Recursively collects secrets for a service and its inherited parents.
// Child secrets overwrite parent secrets on key collisions.
func collectSecrets(ctx context.Context, service string, secretStore store.Store, visited map[string]bool, merged map[string]store.Secret) error {
	if visited[service] {
		return nil
	}
	visited[service] = true

	metadataStore, err := getMetadataStore(ctx)
	if err != nil {
		return fmt.Errorf("Failed to get secret store: %w", err)
	}

	// Read metadata to get children
	metadata, err := metadataStore.Read(ctx, service)
	if err != nil {
		return fmt.Errorf("failed to read metadata for %q: %w", service, err)
	}

	// Recurse into children first
	for _, child := range metadata.Inherits {
		if err := collectSecrets(ctx, child, secretStore, visited, merged); err != nil {
			return err
		}
	}

	// Now add this service's secrets, overwriting any child secrets
	secrets, err := secretStore.List(ctx, service, true)
	if err != nil {
		return fmt.Errorf("failed to list secrets for %q: %w", service, err)
	}

	for _, s := range secrets {
		merged[key(s.Meta.Key)] = s // parent overwrites child
	}

	return nil
}

// Flatten and deduplicate by key, with child overwriting parent
func mergeSecretsWithInheritance(secrets []store.Secret) []store.Secret {
	merged := make(map[string]store.Secret)

	// Iterate in order: parents first, children last
	// Only set if key not already present (parents first)
	for i := len(secrets) - 1; i >= 0; i-- {
		s := secrets[i]
		merged[key(s.Meta.Key)] = s
	}

	// Convert back to slice
	result := make([]store.Secret, 0, len(merged))
	for _, s := range merged {
		result = append(result, s)
	}

	return result
}

func key(s string) string {
	sep := "/"

	tokens := strings.Split(s, sep)
	secretKey := tokens[len(tokens)-1]
	return secretKey
}

func serviceName(s string) string {
	sep := "/"

	tokens := strings.Split(s, sep)
	secretKey := strings.Join(tokens[0:len(tokens)-1], sep)
	return secretKey
}

type ByName []store.Secret

func (a ByName) Len() int           { return len(a) }
func (a ByName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByName) Less(i, j int) bool { return a[i].Meta.Key < a[j].Meta.Key }

type ByTime []store.Secret

func (a ByTime) Len() int           { return len(a) }
func (a ByTime) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByTime) Less(i, j int) bool { return a[i].Meta.Created.Before(a[j].Meta.Created) }

type ByUser []store.Secret

func (a ByUser) Len() int           { return len(a) }
func (a ByUser) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByUser) Less(i, j int) bool { return a[i].Meta.CreatedBy < a[j].Meta.CreatedBy }

type ByVersion []store.Secret

func (a ByVersion) Len() int           { return len(a) }
func (a ByVersion) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByVersion) Less(i, j int) bool { return a[i].Meta.Version < a[j].Meta.Version }
