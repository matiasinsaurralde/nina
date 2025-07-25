// Package main provides the CLI entry point for the Nina application.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/matiasinsaurralde/nina/pkg/cli"
	"github.com/matiasinsaurralde/nina/pkg/config"
	"github.com/matiasinsaurralde/nina/pkg/logger"
	"github.com/spf13/cobra"
)

var (
	configPath string
	logLevel   string
	logFormat  string
	verbose    bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "nina",
		Short: "Nina - Container Provisioning Engine CLI",
		Long: `Nina is a Proof of Concept container provisioning engine.
This CLI allows you to interact with the Nina Engine server to manage container deployments.`,
		SilenceUsage:  true, // Don't show usage on errors
		SilenceErrors: true, // Don't show error messages (we handle them ourselves)
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Path to configuration file")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVar(&logFormat, "log-format", "text", "Log format (text, json)")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Enable verbose logging")

	// Add subcommands
	rootCmd.AddCommand(deployCmd())
	rootCmd.AddCommand(buildCmd())
	rootCmd.AddCommand(deleteCmd())
	rootCmd.AddCommand(statusCmd())
	rootCmd.AddCommand(listCmd())
	rootCmd.AddCommand(healthCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func getCLI() (*cli.CLI, *logger.Logger, error) {
	// Set log level based on verbose flag
	if verbose {
		logLevel = "debug"
	}

	// Initialize logger
	log := logger.New(logger.Level(logLevel), logFormat)
	log.ForceColor() // Force color output for better visibility

	// Load configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Initialize CLI
	c := cli.NewCLI(cfg, log)
	return c, log, nil
}

func deployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy applications",
		Long:  `Deploy applications. Use 'deploy' to deploy the current directory, 'deploy ls' to list deployments, or 'deploy rm' to remove deployments.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			c, log, err := getCLI()
			if err != nil {
				return err
			}

			// Get current working directory
			workingDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current working directory: %w", err)
			}

			log.Info("Deploying project from directory", "dir", workingDir)

			startTime := time.Now()
			deployment, err := c.Deploy(context.Background(), workingDir)
			if err != nil {
				return fmt.Errorf("failed to deploy application: %w", err)
			}

			elapsed := time.Since(startTime)

			// Output friendly success message
			fmt.Printf("✅ Deployment completed successfully!\n")
			fmt.Printf("🆔 Deployment ID: %s\n", deployment.ID)
			fmt.Printf("📱 App Name: %s\n", deployment.AppName)
			fmt.Printf("🔗 Commit Hash: %s\n", deployment.CommitHash)
			fmt.Printf("👤 Author: %s\n", deployment.Author)
			fmt.Printf("📝 Commit Message: %s\n", deployment.CommitMessage)
			fmt.Printf("📊 Status: %s\n", deployment.Status)
			fmt.Printf("⏱️  Elapsed Time: %s\n", elapsed)

			if len(deployment.Containers) > 0 {
				fmt.Printf("🐳 Containers:\n")
				for i, container := range deployment.Containers {
					fmt.Printf("  %d. ID: %s, Image: %s, Address: %s:%d\n",
						i+1, container.ContainerID, container.ImageTag, container.Address, container.Port)
				}
			}

			fmt.Printf("\nThe application has been successfully deployed.\n")
			return nil
		},
	}

	// Add subcommands
	cmd.AddCommand(deployLsCmd())
	cmd.AddCommand(deployRmCmd())

	return cmd
}

func deployLsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List all deployments",
		Long:  `List all deployments in a tabular format.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			c, log, err := getCLI()
			if err != nil {
				return err
			}

			log.Info("Listing deployments")

			deployments, err := c.ListDeployments(context.Background())
			if err != nil {
				return fmt.Errorf("failed to list deployments: %w", err)
			}

			if len(deployments) == 0 {
				fmt.Println("No deployments found.")
				return nil
			}

			// Print header
			fmt.Printf("%-20s %-12s %-20s %-40s %-15s\n", "APP NAME", "COMMIT HASH", "AUTHOR", "COMMIT MESSAGE", "STATUS")
			fmt.Println(strings.Repeat("-", 110))

			// Print deployments
			for _, deployment := range deployments {
				// Truncate commit message if too long
				commitMsg := deployment.CommitMessage
				if len(commitMsg) > 37 {
					commitMsg = commitMsg[:37] + "..."
				}

				// Truncate commit hash to 12 characters
				commitHash := deployment.CommitHash
				if len(commitHash) > 12 {
					commitHash = commitHash[:12]
				}

				fmt.Printf("%-20s %-12s %-20s %-40s %-15s\n",
					deployment.AppName,
					commitHash,
					deployment.Author,
					commitMsg,
					deployment.Status)
			}

			fmt.Printf("\nTotal deployments: %d\n", len(deployments))
			return nil
		},
	}

	return cmd
}

func deployRmCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm [id]",
		Short: "Remove deployments by ID",
		Long:  `Remove deployments by ID. This will delete the deployment with the given ID.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			c, _, err := getCLI()
			if err != nil {
				return err
			}
			id := args[0]
			url := fmt.Sprintf("http://%s/api/v1/deployments/%s", c.Config().GetServerAddr(), id)
			req, err := http.NewRequest("DELETE", url, nil)
			if err != nil {
				return fmt.Errorf("failed to create request: %w", err)
			}
			resp, err := c.Client().Do(req)
			if err != nil {
				return fmt.Errorf("failed to send request: %w", err)
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("failed to read response: %w", err)
			}
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("delete failed: %s (status: %d)", string(body), resp.StatusCode)
			}
			fmt.Printf("Deployment %s deleted successfully\n", id)
			return nil
		},
	}
	return cmd
}

func buildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build projects",
		Long:  `Build projects. Use 'build' to create a new build from the current directory, or 'build ls' to list existing builds.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			c, log, err := getCLI()
			if err != nil {
				return err
			}

			// Get current working directory
			workingDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current working directory: %w", err)
			}

			log.Info("Building project from directory", "dir", workingDir)

			builtImage, err := c.Build(context.Background(), workingDir)
			if err != nil {
				return fmt.Errorf("failed to build deployment: %w", err)
			}

			// Output friendly success message
			fmt.Printf("✅ Build completed successfully!\n")
			fmt.Printf("📦 Image Tag: %s\n", builtImage.ImageTag)
			fmt.Printf("🆔 Image ID: %s\n", builtImage.ImageID)
			fmt.Printf("📏 Size: %s\n", formatBytes(builtImage.Size))
			fmt.Printf("\nThe container image has been successfully built and stored.\n")
			return nil
		},
	}

	// Add subcommands
	cmd.AddCommand(buildLsCmd())
	cmd.AddCommand(buildRmCmd())

	return cmd
}

func buildLsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List all builds",
		Long:  `List all builds in a tabular format.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			c, log, err := getCLI()
			if err != nil {
				return err
			}

			log.Info("Listing builds")

			builds, err := c.ListBuilds(context.Background())
			if err != nil {
				return fmt.Errorf("failed to list builds: %w", err)
			}

			if len(builds) == 0 {
				fmt.Println("No builds found.")
				return nil
			}

			// Print header
			fmt.Printf("%-20s %-12s %-20s %-40s %-15s\n", "APP NAME", "COMMIT HASH", "AUTHOR", "COMMIT MESSAGE", "STATUS")
			fmt.Println(strings.Repeat("-", 110))

			// Print builds
			for _, build := range builds {
				// Truncate commit message if too long
				commitMsg := build.CommitMessage
				if len(commitMsg) > 37 {
					commitMsg = commitMsg[:37] + "..."
				}

				// Truncate commit hash to 12 characters
				commitHash := build.CommitHash
				if len(commitHash) > 12 {
					commitHash = commitHash[:12]
				}

				fmt.Printf("%-20s %-12s %-20s %-40s %-15s\n",
					build.AppName,
					commitHash,
					build.Author,
					commitMsg,
					build.Status)
			}

			fmt.Printf("\nTotal builds: %d\n", len(builds))
			return nil
		},
	}

	return cmd
}

func buildRmCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm [id]",
		Short: "Remove builds by app name or commit hash",
		Long:  `Remove builds by app name or commit hash. This will delete all builds that match the given app name or commit hash.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			c, _, err := getCLI()
			if err != nil {
				return err
			}
			id := args[0]
			url := fmt.Sprintf("http://%s/api/v1/builds/%s", c.Config().GetServerAddr(), id)
			req, err := http.NewRequest("DELETE", url, nil)
			if err != nil {
				return fmt.Errorf("failed to create request: %w", err)
			}
			resp, err := c.Client().Do(req)
			if err != nil {
				return fmt.Errorf("failed to send request: %w", err)
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("failed to read response: %w", err)
			}
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("delete failed: %s (status: %d)", string(body), resp.StatusCode)
			}
			var response struct {
				Deleted []string `json:"deleted"`
				Count   int      `json:"count"`
			}
			if err := json.Unmarshal(body, &response); err != nil {
				return fmt.Errorf("failed to unmarshal response: %w", err)
			}
			if response.Count == 0 {
				fmt.Printf("No builds matched '%s'.\n", id)
				return nil
			}
			fmt.Printf("Deleted %d build(s):\n", response.Count)
			for _, key := range response.Deleted {
				fmt.Printf("- %s\n", key)
			}
			return nil
		},
	}
	return cmd
}

func deleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete [deployment-id]",
		Short: "Delete a deployment",
		Long:  `Delete a deployment by its ID.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			c, log, err := getCLI()
			if err != nil {
				return err
			}

			id := args[0]
			log.Info("Deleting deployment", "id", id)

			if err := c.DeleteDeployment(context.Background(), id); err != nil {
				return fmt.Errorf("failed to delete deployment: %w", err)
			}

			fmt.Printf("Deployment %s deleted successfully\n", id)
			return nil
		},
	}

	return cmd
}

func statusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status [deployment-id]",
		Short: "Get deployment status",
		Long:  `Get the status of a deployment by its ID.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			c, log, err := getCLI()
			if err != nil {
				return err
			}

			id := args[0]
			log.Info("Getting deployment status", "id", id)

			deployment, err := c.GetDeploymentStatus(context.Background(), id)
			if err != nil {
				return fmt.Errorf("failed to get deployment status: %w", err)
			}

			// Output JSON
			data, err := json.MarshalIndent(deployment, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal response: %w", err)
			}

			fmt.Println(string(data))
			return nil
		},
	}

	return cmd
}

func listCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all deployments",
		Long:  `List all deployments in a tabular format.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			c, log, err := getCLI()
			if err != nil {
				return err
			}

			log.Info("Listing deployments")

			deployments, err := c.ListDeployments(context.Background())
			if err != nil {
				return fmt.Errorf("failed to list deployments: %w", err)
			}

			if len(deployments) == 0 {
				fmt.Println("No deployments found.")
				return nil
			}

			// Print header
			fmt.Printf("%-20s %-12s %-20s %-40s %-15s\n", "APP NAME", "COMMIT HASH", "AUTHOR", "COMMIT MESSAGE", "STATUS")
			fmt.Println(strings.Repeat("-", 110))

			// Print deployments
			for _, deployment := range deployments {
				// Truncate commit message if too long
				commitMsg := deployment.CommitMessage
				if len(commitMsg) > 37 {
					commitMsg = commitMsg[:37] + "..."
				}

				// Truncate commit hash to 12 characters
				commitHash := deployment.CommitHash
				if len(commitHash) > 12 {
					commitHash = commitHash[:12]
				}

				fmt.Printf("%-20s %-12s %-20s %-40s %-15s\n",
					deployment.AppName,
					commitHash,
					deployment.Author,
					commitMsg,
					deployment.Status)
			}

			fmt.Printf("\nTotal deployments: %d\n", len(deployments))
			return nil
		},
	}

	return cmd
}

func healthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Check Engine server health",
		Long:  `Check if the Engine server is healthy and responding.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			c, log, err := getCLI()
			if err != nil {
				return err
			}

			log.Info("Checking Engine server health")

			if err := c.HealthCheck(context.Background()); err != nil {
				return fmt.Errorf("health check failed: %w", err)
			}

			fmt.Println("✅ Engine server is healthy")
			return nil
		},
	}

	return cmd
}

// formatBytes formats bytes into a human-readable string
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
