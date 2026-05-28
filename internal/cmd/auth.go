package cmd

import (
	"fmt"

	"github.com/agynio/outline-cli/internal/auth"
	"github.com/agynio/outline-cli/internal/config"
	"github.com/agynio/outline-cli/internal/outline"
	"github.com/spf13/cobra"
)

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "auth", Short: "Authentication commands"}
	cmd.AddCommand(newAuthLoginCmd())
	cmd.AddCommand(newAuthLogoutCmd())
	cmd.AddCommand(newAuthInfoCmd())
	cmd.AddCommand(newAuthConfigCmd())
	return cmd
}

func newAuthLoginCmd() *cobra.Command {
	var loginBaseURL string
	var apiKey string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Save Outline API credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			normalizedBaseURL, err := config.NormalizeBaseURL(loginBaseURL)
			if err != nil {
				return err
			}
			key, err := trimRequired(apiKey, "api key")
			if err != nil {
				return err
			}

			runContext, err := RunContextFrom(cmd)
			if err != nil {
				return err
			}
			updatedConfig := loginConfig(runContext.Config, normalizedBaseURL)

			if err := config.Save(&updatedConfig); err != nil {
				return err
			}
			if err := auth.SaveToken(key); err != nil {
				return err
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Saved Outline credentials for %s\n", normalizedBaseURL)
			return err
		},
	}
	cmd.Flags().StringVar(&loginBaseURL, "base-url", "", "Outline base URL")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "Outline API key")
	_ = cmd.MarkFlagRequired("base-url")
	_ = cmd.MarkFlagRequired("api-key")
	return cmd
}

func loginConfig(cfg *config.Config, normalizedBaseURL string) config.Config {
	updatedConfig := config.Config{Output: config.DefaultOutput}
	if cfg != nil {
		updatedConfig = *cfg
	}
	updatedConfig.BaseURL = normalizedBaseURL
	return updatedConfig
}

func newAuthLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove saved Outline API credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := auth.DeleteToken(); err != nil {
				return err
			}
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "Removed Outline credentials")
			return err
		},
	}
}

func newAuthInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show authenticated Outline user and workspace",
		RunE: func(cmd *cobra.Command, args []string) error {
			runContext, err := RunContextFrom(cmd)
			if err != nil {
				return err
			}
			response, err := runContext.Client.Post(cmd.Context(), "auth.info", nil)
			if err != nil {
				return err
			}
			return printResponse(cmd, outline.ResponseData(response))
		},
	}
}

func newAuthConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Show Outline deployment auth configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			runContext, err := RunContextFrom(cmd)
			if err != nil {
				return err
			}
			response, err := runContext.Client.Post(cmd.Context(), "auth.config", nil)
			if err != nil {
				return err
			}
			return printResponse(cmd, outline.ResponseData(response))
		},
	}
}

func init() {
	rootCmd.AddCommand(newAuthCmd())
}
