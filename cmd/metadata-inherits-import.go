package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// listCmd represents the list command
var metadataInheritsImportCmd = &cobra.Command{
	Use:   "import [file]",
	Short: "import inheritance metadata for one or more services from a file",
	Args:  cobra.ExactArgs(1),
	RunE:  inheritsImport,
}

func init() {
	metadataInheritsCmd.AddCommand(metadataInheritsImportCmd)
}

type ImportSpec map[string]struct {
	Inherits []string `yaml:"inherits"`
}

func inheritsImport(cmd *cobra.Command, args []string) error {
	file := args[0]
	bytes, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	var imports ImportSpec
	if err := yaml.Unmarshal(bytes, &imports); err != nil {
		return err
	}

	metadataStore, err := getMetadataStore(cmd.Context())
	if err != nil {
		return fmt.Errorf("Failed to get metadata store: %w", err)
	}

	for service, def := range imports {
		if err := wouldCreateCycle(cmd.Context(), metadataStore, service, def.Inherits); err != nil {
			return err
		}
		metadataStore.SetInherits(cmd.Context(), service, def.Inherits)
	}

	return nil
}
