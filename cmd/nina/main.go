// Package main provides the CLI entry point for the Nina application.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/matiasinsaurralde/nina/pkg/cli"
	"github.com/matiasinsaurralde/nina/pkg/config"
	"github.com/matiasinsaurralde/nina/pkg/logger"
	"github.com/matiasinsaurralde/nina/pkg/store"
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
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Path to configuration file")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVar(&logFormat, "log-format", "text", "Log format (text, json)")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Enable verbose logging")

	// Add subcommands
	rootCmd.AddCommand(provisionCmd())
	rootCmd.AddCommand(buildCmd())
	rootCmd.AddCommand(buildsCmd())
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

func provisionCmd() *cobra.Command {
	var (
		name        string
		image       string
		ports       []int
		environment map[string]string
	)

	cmd := &cobra.Command{
		Use:   "provision",
		Short: "Provision a new container deployment",
		Long:  `Provision a new container deployment with the specified configuration.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			c, log, err := getCLI()
			if err != nil {
				return err
			}

			if name == "" || image == "" {
				return fmt.Errorf("name and image are required")
			}

			req := &store.ProvisionRequest{
				Name:        name,
				Image:       image,
				Ports:       ports,
				Environment: environment,
			}

			log.Info("Provisioning deployment", "name", name, "image", image)

			deployment, err := c.Provision(context.Background(), req)
			if err != nil {
				return fmt.Errorf("failed to provision deployment: %w", err)
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

	cmd.Flags().StringVar(&name, "name", "", "Deployment name")
	cmd.Flags().StringVar(&image, "image", "", "Container image")
	cmd.Flags().IntSliceVar(&ports, "ports", []int{}, "Container ports")
	cmd.Flags().StringToStringVar(&environment, "env", map[string]string{}, "Environment variables")

	if err := cmd.MarkFlagRequired("name"); err != nil {
		panic(err) // This should never happen in practice
	}
	if err := cmd.MarkFlagRequired("image"); err != nil {
		panic(err) // This should never happen in practice
	}

	return cmd
}

func buildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build a deployment from the current directory",
		Long:  `Build a deployment from the current Git repository. This command will create a TAR archive of the current directory (excluding .git), compress it, and send it to the Engine server for building.`,
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

			log.Info("Building deployment from directory", "dir", workingDir)

			deployment, err := c.Build(context.Background(), workingDir)
			if err != nil {
				return fmt.Errorf("failed to build deployment: %w", err)
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

func buildsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "builds",
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
		Long:  `List all deployments.`,
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

			// Output JSON
			data, err := json.MarshalIndent(deployments, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal response: %w", err)
			}

			fmt.Println(string(data))
			return nil
		},
	}

	return cmd
}

func healthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Check Engine server health",
		Long:  `Check if the Engine server is healthy.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			c, log, err := getCLI()
			if err != nil {
				return err
			}

			log.Info("Checking Engine server health")

			if err := c.HealthCheck(context.Background()); err != nil {
				return fmt.Errorf("health check failed: %w", err)
			}

			fmt.Println("Engine server is healthy")
			return nil
		},
	}

	return cmd
}
