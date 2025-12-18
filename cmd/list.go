package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"

	analytics "github.com/segmentio/analytics-go/v3"
	"github.com/segmentio/chamber/v3/store"
	"github.com/segmentio/chamber/v3/utils"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

type serviceCache struct {
	metadata store.Metadata
	secrets  []store.Secret
}

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
	metadataStore, err := getMetadataStore(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to get metadata store: %w", err)
	}

	// visited := make(map[string]bool)
	merged := make(map[string]store.Secret)
	// cacheMeta := make(map[string]*store.Metadata)
	// cacheSecrets := make(map[string][]store.Secret)

	// var cache = make(map[string]*serviceCache)
	merged, err = collectSecretsConcurrent2(cmd.Context(), service, secretStore, metadataStore)
	// err = collectSecrets(cmd.Context(), service, secretStore, metadataStore, visited, merged, cache)
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

func collectSecretsConcurrent2(
	ctx context.Context,
	rootService string,
	secretStore store.Store,
	metadataStore store.MetadataStore,
) (map[string]store.Secret, error) {

	// -------------------------
	// Phase 1: Parallel metadata discovery
	// -------------------------

	type node struct {
		children []string
	}

	graph := make(map[string]*node)
	visited := make(map[string]bool)

	var mu sync.Mutex
	g, _ := errgroup.WithContext(ctx)

	// Limit concurrent metadata reads
	sem := make(chan struct{}, 8)

	var fetchMetadata func(string) error
	fetchMetadata = func(svc string) error {
		mu.Lock()
		if visited[svc] {
			mu.Unlock()
			return nil
		}
		visited[svc] = true
		mu.Unlock()

		sem <- struct{}{}
		metadata, err := metadataStore.Read(ctx, svc)
		<-sem
		if err != nil {
			return fmt.Errorf("failed to read metadata for %q: %w", svc, err)
		}

		mu.Lock()
		graph[svc] = &node{children: metadata.Inherits}
		mu.Unlock()

		for _, child := range metadata.Inherits {
			g.Go(func() error {
				return fetchMetadata(child)
			})
		}
		return nil
	}

	if err := fetchMetadata(rootService); err != nil {
		return nil, err
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	// -------------------------
	// Phase 2: Deterministic post-order traversal
	// -------------------------

	services := []string{}
	seen := make(map[string]bool)

	var postOrder func(string)
	postOrder = func(svc string) {
		if seen[svc] {
			return
		}
		seen[svc] = true

		for _, child := range graph[svc].children {
			postOrder(child)
		}
		services = append(services, svc)
	}

	postOrder(rootService)

	// -------------------------
	// Phase 3: Parallel secret listing
	// -------------------------

	results := make(map[string][]store.Secret)
	var resultsMu sync.Mutex
	g, _ = errgroup.WithContext(ctx)

	for _, svc := range services {
		g.Go(func() error {
			secrets, err := secretStore.List(ctx, svc, true)
			if err != nil {
				return fmt.Errorf("failed to list secrets for %q: %w", svc, err)
			}

			resultsMu.Lock()
			results[svc] = secrets
			resultsMu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// -------------------------
	// Phase 4: Ordered merge (parents overwrite children)
	// -------------------------

	merged := make(map[string]store.Secret)
	for _, svc := range services {
		for _, s := range results[svc] {
			merged[key(s.Meta.Key)] = s
		}
	}

	return merged, nil
}

func collectSecretsConcurrent(ctx context.Context, rootService string, secretStore store.Store, metadataStore store.MetadataStore) (map[string]store.Secret, error) {
	// Step 1: Traverse tree to collect unique services
	visited := make(map[string]bool)
	var collectServices func(svc string) error
	services := []string{}

	collectServices = func(svc string) error {
		if visited[svc] {
			return nil
		}
		visited[svc] = true

		metadata, err := metadataStore.Read(ctx, svc)
		if err != nil {
			return fmt.Errorf("failed to read metadata for %q: %w", svc, err)
		}

		for _, child := range metadata.Inherits {
			if err := collectServices(child); err != nil {
				return err
			}
		}

		services = append(services, svc) // append after children for bottom-up merging
		return nil
	}

	if err := collectServices(rootService); err != nil {
		return nil, err
	}

	// Step 2: Fetch all secrets concurrently
	results := make(map[string][]store.Secret)
	var mu sync.Mutex
	g, ctx := errgroup.WithContext(ctx)

	for _, svc := range services {
		g.Go(func() error {
			secrets, err := secretStore.List(ctx, svc, true)
			if err != nil {
				return fmt.Errorf("failed to list secrets for %q: %w", svc, err)
			}

			mu.Lock()
			results[svc] = secrets
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Step 3: Merge secrets bottom-up (parents overwrite children)
	merged := make(map[string]store.Secret)
	// Merge children first, parents last
	for _, svc := range services {
		for _, s := range results[svc] {
			merged[key(s.Meta.Key)] = s
		}
	}
	return merged, nil
}

func collectSecrets(ctx context.Context, service string, secretStore store.Store, metadataStore store.MetadataStore, visited map[string]bool, merged map[string]store.Secret, cache map[string]*serviceCache) error {
	if visited[service] {
		return nil
	}
	visited[service] = true

	// Check cache
	if _, ok := cache[service]; !ok {
		metadata, err := metadataStore.Read(ctx, service)
		if err != nil {
			return fmt.Errorf("failed to read metadata for %q: %w", service, err)
		}

		secrets, err := secretStore.List(ctx, service, true)
		if err != nil {
			return fmt.Errorf("failed to list secrets for %q: %w", service, err)
		}

		cache[service] = &serviceCache{
			metadata: metadata,
			secrets:  secrets,
		}
	} else {
		fmt.Println("cache hit:", service)
	}

	// Recurse into children concurrently
	g, ctx := errgroup.WithContext(ctx)
	for _, child := range cache[service].metadata.Inherits {
		child := child
		g.Go(func() error {
			return collectSecrets(ctx, child, secretStore, metadataStore, visited, merged, cache)
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	// Merge this service's secrets (parent overwrites children)
	for _, s := range cache[service].secrets {
		merged[key(s.Meta.Key)] = s
	}

	return nil
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
