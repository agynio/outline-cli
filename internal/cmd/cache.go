package cmd

import (
	"fmt"

	"github.com/agynio/outline-cli/internal/output"
	"github.com/spf13/cobra"
)

func newCacheCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "cache", Short: "Manage local Outline cache"}
	cmd.AddCommand(newCacheShowCmd())
	cmd.AddCommand(newCacheClearCmd())
	return cmd
}

func newCacheShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show local cache",
		RunE: func(cmd *cobra.Command, args []string) error {
			cache, err := loadShareCache()
			if err != nil {
				return err
			}
			runContext, err := RunContextFrom(cmd)
			if err != nil {
				return err
			}
			return output.Print(cmd.OutOrStdout(), runContext.OutputFormat, cache)
		},
	}
}

func newCacheClearCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Clear local cache",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := clearShareCache(); err != nil {
				return err
			}
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "Cleared Outline cache")
			return err
		},
	}
}
