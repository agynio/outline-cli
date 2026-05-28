package cmd

import (
	"fmt"
	"os"
	"strings"

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
	var documentID string

	cmd := &cobra.Command{
		Use:   "info",
		Short: "Show document details",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRPC(cmd, "documents.info", map[string]any{"id": documentID})
		},
	}
	cmd.Flags().StringVar(&documentID, "id", "", "Document ID or urlId")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func newDocumentsPullCmd() *cobra.Command {
	var documentID string

	cmd := &cobra.Command{
		Use:   "pull [id-or-urlId]",
		Short: "Print document Markdown",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedID, err := idFromFlagOrArg(documentID, args, "document ID or urlId")
			if err != nil {
				return err
			}
			runContext, err := RunContextFrom(cmd)
			if err != nil {
				return err
			}
			response, err := runContext.Client.Post(cmd.Context(), "documents.info", map[string]any{"id": resolvedID})
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
	cmd.Flags().StringVar(&documentID, "id", "", "Document ID or urlId")
	return cmd
}

func newDocumentsListCmd() *cobra.Command {
	var collectionID string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List documents",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedCollectionID, err := aliasedStringFlagValue(cmd, "collection", "collection-id")
			if err != nil {
				return err
			}
			payload := map[string]any{}
			if resolvedCollectionID != "" {
				payload["collectionId"] = resolvedCollectionID
			}
			return runRPC(cmd, "documents.list", payload)
		},
	}
	cmd.Flags().StringVar(&collectionID, "collection", "", "Collection ID")
	cmd.Flags().String("collection-id", "", "Collection ID (alias)")
	return cmd
}

func newDocumentsSearchCmd() *cobra.Command {
	var collectionID string

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search documents",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedCollectionID, err := aliasedStringFlagValue(cmd, "collection", "collection-id")
			if err != nil {
				return err
			}
			payload := map[string]any{"query": args[0]}
			if resolvedCollectionID != "" {
				payload["collectionId"] = resolvedCollectionID
			}
			return runRPC(cmd, "documents.search", payload)
		},
	}
	cmd.Flags().StringVar(&collectionID, "collection", "", "Collection ID")
	cmd.Flags().String("collection-id", "", "Collection ID (alias)")
	return cmd
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
			resolvedCollectionID, err := aliasedStringFlagValue(cmd, "collection", "collection-id")
			if err != nil {
				return err
			}
			markdown, err := markdownInput(filePath, text)
			if err != nil {
				return err
			}
			payload := map[string]any{
				"collectionId": resolvedCollectionID,
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
	cmd.Flags().String("collection-id", "", "Collection ID (alias)")
	cmd.Flags().StringVar(&title, "title", "", "Document title")
	cmd.Flags().StringVar(&filePath, "file", "", "Markdown file")
	cmd.Flags().StringVar(&text, "text", "", "Markdown text")
	cmd.Flags().BoolVar(&publish, "publish", false, "Publish document")
	cmd.MarkFlagsOneRequired("collection", "collection-id")
	_ = cmd.MarkFlagRequired("title")
	return cmd
}

func newDocumentsUpdateCmd() *cobra.Command {
	var documentID string
	var collectionID string
	var filePath string
	var text string

	cmd := &cobra.Command{
		Use:   "update [id]",
		Short: "Update a document",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedID, err := idFromFlagOrArg(documentID, args, "document ID")
			if err != nil {
				return err
			}

			trimmedCollectionID, err := aliasedStringFlagValue(cmd, "collection", "collection-id")
			if err != nil {
				return err
			}
			if filePath == "" && text == "" && trimmedCollectionID == "" {
				return fmt.Errorf("one of --file, --text, or --collection-id is required")
			}

			payload := map[string]any{"id": resolvedID}
			if filePath != "" || text != "" {
				markdown, err := markdownInput(filePath, text)
				if err != nil {
					return err
				}
				payload["text"] = markdown
			}
			if trimmedCollectionID != "" {
				payload["collectionId"] = trimmedCollectionID
			}
			return runRPC(cmd, "documents.update", payload)
		},
	}
	cmd.Flags().StringVar(&documentID, "id", "", "Document ID")
	cmd.Flags().StringVar(&collectionID, "collection", "", "Collection ID")
	cmd.Flags().String("collection-id", "", "Collection ID (alias)")
	cmd.Flags().StringVar(&filePath, "file", "", "Markdown file")
	cmd.Flags().StringVar(&text, "text", "", "Markdown text")
	return cmd
}

func idFromFlagOrArg(flagValue string, args []string, label string) (string, error) {
	trimmedFlag := strings.TrimSpace(flagValue)
	if len(args) == 0 {
		if trimmedFlag == "" {
			return "", fmt.Errorf("--id or %s argument is required", label)
		}
		return trimmedFlag, nil
	}
	trimmedArg := strings.TrimSpace(args[0])
	if trimmedArg == "" {
		return "", fmt.Errorf("%s argument is required", label)
	}
	if trimmedFlag != "" && trimmedFlag != trimmedArg {
		return "", fmt.Errorf("use either --id or %s argument, not both", label)
	}
	return trimmedArg, nil
}

func aliasedStringFlagValue(cmd *cobra.Command, name string, alias string) (string, error) {
	canonicalChanged := cmd.Flags().Changed(name)
	aliasChanged := cmd.Flags().Changed(alias)
	if !canonicalChanged && !aliasChanged {
		return "", nil
	}
	canonicalValue, err := cmd.Flags().GetString(name)
	if err != nil {
		return "", err
	}
	aliasValue, err := cmd.Flags().GetString(alias)
	if err != nil {
		return "", err
	}
	trimmedCanonical := strings.TrimSpace(canonicalValue)
	trimmedAlias := strings.TrimSpace(aliasValue)
	if canonicalChanged && aliasChanged {
		if trimmedCanonical != trimmedAlias {
			return "", fmt.Errorf("conflicting values for --%s and --%s", name, alias)
		}
		return trimmedCanonical, nil
	}
	if canonicalChanged {
		return trimmedCanonical, nil
	}
	return trimmedAlias, nil
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
}
