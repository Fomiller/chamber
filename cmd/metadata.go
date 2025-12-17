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

	metadataInheritCmd = &cobra.Command{
		Use:   "inherit <subcommand>",
		Short: "work with inherit metadata for a service",
		Args:  cobra.ExactArgs(1),
		RunE:  inherit,
	}
)

func init() {
	RootCmd.AddCommand(metadataCmd)
	metadataCmd.AddCommand(metadataInheritCmd)
}
