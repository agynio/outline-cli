package cmd

import (
	"fmt"
	"os"

	"github.com/agynio/outline-cli/internal/outline"
	"github.com/agynio/outline-cli/internal/output"
	"github.com/spf13/cobra"
)

func newDocumentsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "documents", Short: "Manage Outline documents"}
	cmd.AddCommand(newDocumentsInfoCmd())
	cmd.AddCommand(newDocumentsPullCmd())
	cmd.AddCommand(newDocumentsListCmd())
	cmd.AddCommand(newDocumentsSearchCmd())
	cmd.AddCommand(newDocumentsCreateCmd())
	cmd.AddCommand(newDocumentsUpdateCmd())
	return cmd
}

func newDocumentsInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info <id-or-urlId>",
		Short: "Show document details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRPC(cmd, "documents.info", map[string]any{"id": args[0]})
		},
	}
}

func newDocumentsPullCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pull <id-or-urlId>",
		Short: "Print document Markdown",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runContext, err := RunContextFrom(cmd)
			if err != nil {
				return err
			}
			response, err := runContext.Client.Post(cmd.Context(), "documents.info", map[string]any{"id": args[0]})
			if err != nil {
				return err
			}
			text, err := outline.DocumentText(response)
			if err != nil {
				return err
			}
			return output.Print(cmd.OutOrStdout(), output.FormatMD, output.Markdown{Text: text})
		},
	}
}

func newDocumentsListCmd() *cobra.Command {
	return rpcCommand("list", "List documents", "documents.list", nil)
}

func newDocumentsSearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Search documents",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRPC(cmd, "documents.search", map[string]any{"query": args[0]})
		},
	}
}

func newDocumentsCreateCmd() *cobra.Command {
	var collectionID string
	var title string
	var filePath string
	var text string
	var publish bool

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a document",
		RunE: func(cmd *cobra.Command, args []string) error {
			markdown, err := markdownInput(filePath, text)
			if err != nil {
				return err
			}
			payload := map[string]any{
				"collectionId": collectionID,
				"title":        title,
				"text":         markdown,
			}
			if publish {
				payload["publish"] = true
			}
			return runRPC(cmd, "documents.create", payload)
		},
	}
	cmd.Flags().StringVar(&collectionID, "collection", "", "Collection ID")
	cmd.Flags().StringVar(&title, "title", "", "Document title")
	cmd.Flags().StringVar(&filePath, "file", "", "Markdown file")
	cmd.Flags().StringVar(&text, "text", "", "Markdown text")
	cmd.Flags().BoolVar(&publish, "publish", false, "Publish document")
	_ = cmd.MarkFlagRequired("collection")
	_ = cmd.MarkFlagRequired("title")
	return cmd
}

func newDocumentsUpdateCmd() *cobra.Command {
	var filePath string
	var text string

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a document",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			markdown, err := markdownInput(filePath, text)
			if err != nil {
				return err
			}
			return runRPC(cmd, "documents.update", map[string]any{"id": args[0], "text": markdown})
		},
	}
	cmd.Flags().StringVar(&filePath, "file", "", "Markdown file")
	cmd.Flags().StringVar(&text, "text", "", "Markdown text")
	return cmd
}

func markdownInput(filePath, text string) (string, error) {
	if filePath != "" && text != "" {
		return "", fmt.Errorf("use either --file or --text, not both")
	}
	if filePath == "" && text == "" {
		return "", fmt.Errorf("one of --file or --text is required")
	}
	if filePath == "" {
		return text, nil
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read markdown file: %w", err)
	}
	return string(data), nil
}

func init() {
	rootCmd.AddCommand(newDocumentsCmd())
}
