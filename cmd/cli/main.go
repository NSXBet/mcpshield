package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	logger  = log.NewWithOptions(os.Stderr, log.Options{
		ReportCaller:    false,
		ReportTimestamp: false,
		Prefix:          "mcpshield",
	})

	// Styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7C3AED")).
			MarginLeft(1)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981")).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444")).
			Bold(true)
)

var rootCmd = &cobra.Command{
	Use:   "mcpshield",
	Short: "MCPShield CLI - Manage authentication and configuration",
	Long:  `MCPShield CLI provides commands to manage authentication and configuration for the MCPShield service.`,
}

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authentication commands",
	Long:  `Commands for managing authentication with MCPShield service.`,
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to MCPShield service",
	Long:  `Authenticate with the MCPShield service and store credentials locally.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(titleStyle.Render("🔐 MCPShield Login"))
		
		// Get config values
		apiEndpoint := viper.GetString("api.endpoint")
		timeout := viper.GetInt("auth.timeout")
		
		logger.Info("Starting authentication process", "endpoint", apiEndpoint, "timeout", timeout)
		
		// TODO: Implement actual login logic
		// - Prompt for credentials or use OAuth flow
		// - Call authentication API
		// - Store tokens securely
		
		fmt.Println(successStyle.Render("✓ Login successful! Credentials stored."))
		
		// Placeholder for actual implementation
		logger.Warn("Login command is not fully implemented yet")
	},
}

var tokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Display or refresh authentication token",
	Long:  `Display the current authentication token or refresh it if expired.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(titleStyle.Render("🔑 MCPShield Token"))
		
		// Get config values
		tokenPath := viper.GetString("auth.token_path")
		refreshThreshold := viper.GetInt("auth.refresh_threshold")
		
		logger.Info("Checking token status", "path", tokenPath, "refresh_threshold", refreshThreshold)
		
		// TODO: Implement actual token logic
		// - Read token from storage
		// - Check expiration
		// - Refresh if needed
		// - Display token info
		
		fmt.Println("Token: <placeholder-token>")
		fmt.Println("Expires: <placeholder-expiry>")
		fmt.Println("Status: " + successStyle.Render("Valid"))
		
		// Placeholder for actual implementation
		logger.Warn("Token command is not fully implemented yet")
	},
}

func init() {
	cobra.OnInitialize(initConfig)
	
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is $HOME/.mcpshield/config.yaml)")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")
	
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	
	authCmd.AddCommand(loginCmd)
	authCmd.AddCommand(tokenCmd)
	rootCmd.AddCommand(authCmd)
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			logger.Error("Failed to get home directory", "error", err)
			os.Exit(1)
		}
		
		defaultConfigPath := filepath.Join(home, ".mcpshield", "config.yaml")
		viper.SetConfigFile(defaultConfigPath)
	}
	
	viper.AutomaticEnv()
	
	// Set defaults (api.endpoint has no default - must be provided)
	viper.SetDefault("api.version", "v1")
	viper.SetDefault("api.timeout", 30)
	viper.SetDefault("auth.timeout", 30)
	viper.SetDefault("auth.token_path", filepath.Join(os.Getenv("HOME"), ".mcpshield", "token"))
	viper.SetDefault("auth.refresh_threshold", 300)
	viper.SetDefault("auth.method", "token")
	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.format", "text")
	viper.SetDefault("log.color", true)
	viper.SetDefault("output.format", "table")
	viper.SetDefault("output.pretty", true)
	viper.SetDefault("output.timestamps", false)
	
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			logger.Error("Config file not found", "path", viper.ConfigFileUsed(), "error", "Please create the config file or specify one with -c")
			os.Exit(1)
		} else {
			logger.Error("Error reading config file", "error", err)
			os.Exit(1)
		}
	} else {
		logger.Debug("Using config file", "file", viper.ConfigFileUsed())
	}
	
	// Validate required configuration
	if viper.GetString("api.endpoint") == "" {
		logger.Error("API endpoint is required", "error", "Please set api.endpoint in your config file")
		os.Exit(1)
	}
	
	// Set log level based on config
	if viper.GetBool("verbose") {
		logger.SetLevel(log.DebugLevel)
	} else {
		switch viper.GetString("log.level") {
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
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(errorStyle.Render("Error: " + err.Error()))
		os.Exit(1)
	}
} 