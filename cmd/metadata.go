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
)

func init() {
	RootCmd.AddCommand(metadataCmd)
}
