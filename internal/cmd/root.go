package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/agynio/outline-cli/internal/auth"
	"github.com/agynio/outline-cli/internal/config"
	"github.com/agynio/outline-cli/internal/outline"
	"github.com/agynio/outline-cli/internal/output"
	"github.com/spf13/cobra"
)

type RunContext struct {
	Config       *config.Config
	Client       *outline.Client
	OutputFormat output.Format
	BaseURL      string
}

type contextKey struct{}

var (
	baseURLFlag string
	outputFlag  string
)

var versionInfo = "dev"

var rootCmd = newRootCmd()

func newRootCmd() *cobra.Command {
	baseURLFlag = ""
	outputFlag = ""
	cmd := &cobra.Command{
		Use:               "outline",
		Short:             "Outline CLI",
		Version:           versionInfo,
		SilenceUsage:      true,
		PersistentPreRunE: rootPersistentPreRun,
	}
	cmd.PersistentFlags().StringVar(&baseURLFlag, "base-url", "", "Outline base URL")
	cmd.PersistentFlags().StringVarP(&outputFlag, "output", "o", "", "Output format: yaml, md, or json")
	cmd.AddCommand(newAPIRootCommands()...)
	return cmd
}

func rootPersistentPreRun(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	formatValue := outputFlag
	if formatValue == "" {
		formatValue = cfg.Output
	}
	format, err := output.ParseFormat(formatValue)
	if err != nil {
		return err
	}

	var client *outline.Client
	var baseURL string
	if requiresBaseURL(cmd, args) {
		resolved, err := config.ResolveBaseURL(cfg, baseURLFlag)
		if err != nil {
			return err
		}
		baseURL = resolved

		token := ""
		if requiresAuth(cmd, args) {
			loadedToken, err := auth.LoadToken(auth.TokenOptions{})
			if err != nil {
				return err
			}
			token = loadedToken
		}
		client = outline.NewClient(baseURL, token)
	}

	cmd.SetContext(withRunContext(cmd.Context(), &RunContext{
		Config:       cfg,
		Client:       client,
		OutputFormat: format,
		BaseURL:      baseURL,
	}))
	return nil
}

func SetVersionInfo(version, commit, date string) {
	versionInfo = fmt.Sprintf("%s (commit %s, built %s)", version, commit, date)
	rootCmd.Version = versionInfo
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func RunContextFrom(cmd *cobra.Command) (*RunContext, error) {
	runContext, ok := cmd.Context().Value(contextKey{}).(*RunContext)
	if !ok || runContext == nil {
		return nil, fmt.Errorf("run context unavailable")
	}
	return runContext, nil
}

func withRunContext(ctx context.Context, runContext *RunContext) context.Context {
	return context.WithValue(ctx, contextKey{}, runContext)
}

func requiresBaseURL(cmd *cobra.Command, args []string) bool {
	if skipPreRun(cmd, args) {
		return false
	}
	if cmd.Name() == "login" && cmd.Parent() != nil && cmd.Parent().Name() == "auth" {
		return false
	}
	if cmd.Name() == "logout" && cmd.Parent() != nil && cmd.Parent().Name() == "auth" {
		return false
	}
	if cmd.Parent() != nil && cmd.Parent().Name() == "cache" {
		return false
	}
	return true
}

func requiresAuth(cmd *cobra.Command, args []string) bool {
	if !requiresBaseURL(cmd, args) {
		return false
	}
	return !(cmd.Name() == "config" && cmd.Parent() != nil && cmd.Parent().Name() == "auth")
}

func skipPreRun(cmd *cobra.Command, args []string) bool {
	if cmd.Name() == "help" || cmd.Flags().Changed("help") {
		return true
	}
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			return true
		}
	}
	return false
}

func printResponse(cmd *cobra.Command, data any) error {
	runContext, err := RunContextFrom(cmd)
	if err != nil {
		return err
	}
	return output.Print(cmd.OutOrStdout(), runContext.OutputFormat, data)
}

func trimRequired(value, name string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return trimmed, nil
}

func init() {
}
