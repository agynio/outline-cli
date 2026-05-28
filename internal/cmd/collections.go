package cmd

import (
	"github.com/agynio/outline-cli/internal/outline"
	"github.com/spf13/cobra"
)

func newCollectionsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "collections", Short: "Manage Outline collections"}
	cmd.AddCommand(newCollectionsListCmd())
	cmd.AddCommand(newCollectionsInfoCmd())
	cmd.AddCommand(newCollectionsTreeCmd())
	return cmd
}

func newCollectionsListCmd() *cobra.Command {
	return rpcCommand("list", "List collections", "collections.list", nil)
}

func newCollectionsInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info <collection-id>",
		Short: "Show collection details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRPC(cmd, "collections.info", map[string]any{"id": args[0]})
		},
	}
}

func newCollectionsTreeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tree <collection-id>",
		Short: "Show collection document tree",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRPC(cmd, "collections.documents", map[string]any{"id": args[0]})
		},
	}
}

func rpcCommand(use, short, method string, payload map[string]any) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRPC(cmd, method, payload)
		},
	}
}

func runRPC(cmd *cobra.Command, method string, payload map[string]any) error {
	runContext, err := RunContextFrom(cmd)
	if err != nil {
		return err
	}
	response, err := runContext.Client.Post(cmd.Context(), method, payload)
	if err != nil {
		return err
	}
	return printResponse(cmd, outline.ResponseData(response))
}

func init() {
}
