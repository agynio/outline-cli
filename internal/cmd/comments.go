package cmd

import "github.com/spf13/cobra"

func newCommentsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "comments", Short: "Manage Outline comments"}
	cmd.AddCommand(newCommentsListCmd())
	cmd.AddCommand(newCommentsCreateCmd())
	return cmd
}

func newCommentsListCmd() *cobra.Command {
	var documentID string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List document comments",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedDocumentID, err := aliasedStringFlagValue(cmd, "document", "document-id")
			if err != nil {
				return err
			}
			return runRPC(cmd, "comments.list", map[string]any{"documentId": resolvedDocumentID})
		},
	}
	cmd.Flags().StringVar(&documentID, "document", "", "Document ID")
	cmd.Flags().String("document-id", "", "Document ID (alias)")
	cmd.MarkFlagsOneRequired("document", "document-id")
	return cmd
}

func newCommentsCreateCmd() *cobra.Command {
	var documentID string
	var text string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a document comment",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedDocumentID, err := aliasedStringFlagValue(cmd, "document", "document-id")
			if err != nil {
				return err
			}
			return runRPC(cmd, "comments.create", map[string]any{"documentId": resolvedDocumentID, "text": text})
		},
	}
	cmd.Flags().StringVar(&documentID, "document", "", "Document ID")
	cmd.Flags().String("document-id", "", "Document ID (alias)")
	cmd.Flags().StringVar(&text, "text", "", "Markdown text")
	cmd.MarkFlagsOneRequired("document", "document-id")
	_ = cmd.MarkFlagRequired("text")
	return cmd
}

func init() {
}
