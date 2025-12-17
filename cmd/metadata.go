package cmd

import (
	"github.com/spf13/cobra"
)

var (
	// tagCmd represents the tag command
	metadataCmd = &cobra.Command{
		Use:   "metadata <subcommand> ...",
		Short: "work with metadata on services",
	}

	metadataInheritsCmd = &cobra.Command{
		Use:   "inherits <subcommand>",
		Short: "work with inherits metadata for a service",
	}
)

func init() {
	RootCmd.AddCommand(metadataCmd)
	metadataCmd.AddCommand(metadataInheritsCmd)
}
