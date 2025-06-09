package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/nsxbet/mcpshield/pkg"
	"github.com/spf13/cobra"
)

var (
	cfgFile string
	logger  = log.NewWithOptions(os.Stderr, log.Options{
		ReportCaller:    false,
		ReportTimestamp: false,
		Prefix:          "mcpshield-server",
	})

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7C3AED")).
			MarginLeft(1)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444")).
			Bold(true)
)

var rootCmd = &cobra.Command{
	Use:   "mcpshield-server",
	Short: "MCPShield Server",
	Long:  `MCPShield Server provides HTTP API endpoints.`,
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Start the server",
	Long:  `Start the HTTP server.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(titleStyle.Render("🚀 Starting MCPShield Server"))
		
		// Determine config file path
		configPath := "/app/config.yaml"
		if cfgFile != "" {
			configPath = cfgFile
		}
		
		// Read configuration
		config, err := pkg.ReadConfig(configPath)
		if err != nil {
			logger.Error("Failed to read config", "error", err, "path", configPath)
			os.Exit(1)
		}
		
		// Set log level from config
		verbose, _ := cmd.Flags().GetBool("verbose")
		if verbose {
			logger.SetLevel(log.DebugLevel)
		} else {
			switch config.GetLogLevel() {
			case "debug":
				logger.SetLevel(log.DebugLevel)
			case "info":
				logger.SetLevel(log.InfoLevel)
			case "warn":
				logger.SetLevel(log.WarnLevel)
			case "error":
				logger.SetLevel(log.ErrorLevel)
			default:
				logger.SetLevel(log.InfoLevel)
			}
		}
		
		logger.Info("Server configuration", "address", config.GetServerAddress(), "namespace", config.GetKubernetesNamespace())
		logger.Debug("Using config file", "file", configPath)
		
		if err := StartServer(config); err != nil {
			logger.Error("Failed to start server", "error", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is /app/config.yaml)")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")
	
	rootCmd.AddCommand(runCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(errorStyle.Render("Error: " + err.Error()))
		os.Exit(1)
	}
} 