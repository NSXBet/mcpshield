package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
)

type MCPProxy struct {
	client    *kubernetes.Clientset
	namespace string
	config    *clientcmd.ClientConfig
	mcpConfig *MCPConfig
}

type MCPConfig struct {
	MCPServers []MCPServer `yaml:"mcp-servers"`
}

type MCPServer struct {
	Name        string            `yaml:"name"`
	Image       string            `yaml:"image"`
	Command     string            `yaml:"command"`
	Args        []string          `yaml:"args"`
	Env         map[string]string `yaml:"env,omitempty"`
}

type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

func main() {
	fmt.Println("🚀 Starting MCP Bridge Proxy...")

	// Initialize Kubernetes client
	client, config, err := createKubernetesClient()
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	// Read MCP configuration
	mcpConfig, err := readMCPConfig("mcp.yaml")
	if err != nil {
		log.Fatalf("Failed to read MCP config: %v", err)
	}

	proxy := &MCPProxy{
		client:    client,
		namespace: "default",
		config:    &config,
		mcpConfig: mcpConfig,
	}

	// Create all MCP server deployments at startup
	err = proxy.initializeMCPServerDeployments()
	if err != nil {
		log.Fatalf("Failed to initialize MCP server deployments: %v", err)
	}

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Setup HTTP routes
	http.HandleFunc("/mcp", proxy.handleMCP)
	http.HandleFunc("/health", handleHealth)

	// Start HTTP server in a goroutine
	server := &http.Server{Addr: ":8080"}
	go func() {
		port := "8080"
		fmt.Printf("✅ MCP Bridge Proxy ready\n")
		fmt.Printf("🌐 MCP endpoint: http://localhost:%s/mcp\n", port)
		fmt.Printf("📍 Kubernetes namespace: %s\n", proxy.namespace)
		
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	fmt.Println("\n🛑 Shutdown signal received, cleaning up...")

	// Cleanup deployments
	if err := proxy.cleanupMCPServerDeployments(); err != nil {
		log.Printf("Error during cleanup: %v", err)
	}

	// Graceful shutdown of HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	fmt.Println("👋 MCP Bridge Proxy stopped")
}

func (p *MCPProxy) handleMCP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request MCPRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Log only for tools/list and tools/call
	if request.Method == "tools/list" {
		log.Printf("📋 tools/list request")
	} else if request.Method == "tools/call" {
		if params, ok := request.Params.(map[string]interface{}); ok {
			if toolName, ok := params["name"].(string); ok {
				log.Printf("🔧 tools/call: %s", toolName)
			}
		}
	}

	// Forward request to MCP server pod
	response, err := p.forwardToMCPServer(&request)
	if err != nil {
		response = &MCPResponse{
			JSONRPC: "2.0",
			ID:      request.ID,
			Error:   map[string]interface{}{"code": -32603, "message": err.Error()},
		}
	}

	// Log tools/call results (both success and error)
	if request.Method == "tools/call" {
		if responseBytes, err := json.Marshal(response); err == nil {
			log.Printf("📤 tools/call result: %s", string(responseBytes))
		}
	}

	json.NewEncoder(w).Encode(response)
}

func (p *MCPProxy) forwardToMCPServer(request *MCPRequest) (*MCPResponse, error) {
	// Determine which server to use based on tool name for tools/call
	serverName := p.mcpConfig.MCPServers[0].Name // Default to first server
	
	if request.Method == "tools/call" {
		if params, ok := request.Params.(map[string]interface{}); ok {
			if toolName, ok := params["name"].(string); ok {
				if extractedServer := p.extractServerName(toolName); extractedServer != "" {
					// Validate server exists in config
					if p.getServerConfig(extractedServer) != nil {
						serverName = extractedServer
					} else {
						return nil, fmt.Errorf("unknown server: %s", extractedServer)
					}
				} else {
					return nil, fmt.Errorf("no server specified in tool name: %s", toolName)
				}
			}
		}
	}

	// Find pod from deployment
	deploymentName := fmt.Sprintf("mcp-%s", serverName)
	podName, err := p.getPodFromDeployment(deploymentName)
	if err != nil {
		return nil, fmt.Errorf("failed to get pod from deployment %s: %w", deploymentName, err)
	}

	// Wait for pod to be ready (in case it's still starting up)
	err = p.waitForPodReady(podName)
	if err != nil {
		return nil, fmt.Errorf("pod not ready: %w", err)
	}

	// Execute MCP request in pod
	response, err := p.executeMCPRequest(podName, serverName, request)
	if err != nil {
		return nil, fmt.Errorf("failed to execute MCP request: %w", err)
	}

	return response, nil
}

func (p *MCPProxy) getServerConfig(serverName string) *MCPServer {
	for _, server := range p.mcpConfig.MCPServers {
		if server.Name == serverName {
			return &server
		}
	}
	return nil
}

func (p *MCPProxy) createMCPServerDeployment(serverName string) (string, error) {
	serverConfig := p.getServerConfig(serverName)
	if serverConfig == nil {
		return "", fmt.Errorf("server not found in config: %s", serverName)
	}

	deploymentName := fmt.Sprintf("mcp-%s", serverName)
	replicas := int32(1)

	envVars := []corev1.EnvVar{}
	for key, value := range serverConfig.Env {
		expandedValue := os.ExpandEnv(value)
		envVars = append(envVars, corev1.EnvVar{
			Name:  key,
			Value: expandedValue,
		})
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: p.namespace,
			Labels: map[string]string{
				"app":    "mcp-bridge",
				"server": serverName,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":    "mcp-bridge",
					"server": serverName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":    "mcp-bridge",
						"server": serverName,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyAlways,
					Containers: []corev1.Container{
						{
							Name:    serverConfig.Name,
							Image:   serverConfig.Image,
							Command: []string{serverConfig.Command},
							Args:    serverConfig.Args,
							Env:     envVars,
							Stdin:   true,
							TTY:     true,
						},
					},
				},
			},
		},
	}

	_, err := p.client.AppsV1().Deployments(p.namespace).Create(
		context.TODO(),
		deployment,
		metav1.CreateOptions{},
	)
	if err != nil {
		return "", err
	}

	return deploymentName, nil
}

func (p *MCPProxy) waitForDeploymentReady(deploymentName string) error {
	timeout := time.After(60 * time.Second)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for deployment %s", deploymentName)
		case <-ticker.C:
			deployment, err := p.client.AppsV1().Deployments(p.namespace).Get(
				context.TODO(),
				deploymentName,
				metav1.GetOptions{},
			)
			if err != nil {
				continue
			}

			if deployment.Status.ReadyReplicas >= *deployment.Spec.Replicas {
				return nil
			}
		}
	}
}

func (p *MCPProxy) deleteDeployment(deploymentName string) error {
	propagationPolicy := metav1.DeletePropagationForeground
	err := p.client.AppsV1().Deployments(p.namespace).Delete(
		context.TODO(),
		deploymentName,
		metav1.DeleteOptions{
			PropagationPolicy: &propagationPolicy,
		},
	)
	return err
}

func (p *MCPProxy) waitForDeploymentDeleted(deploymentName string) error {
	timeout := time.After(60 * time.Second)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for deployment %s to be deleted", deploymentName)
		case <-ticker.C:
			_, err := p.client.AppsV1().Deployments(p.namespace).Get(
				context.TODO(),
				deploymentName,
				metav1.GetOptions{},
			)
			if err != nil {
				// Deployment not found, deletion successful
				return nil
			}
		}
	}
}

func (p *MCPProxy) getPodFromDeployment(deploymentName string) (string, error) {
	// Get pods with the deployment labels
	labelSelector := fmt.Sprintf("app=mcp-bridge,server=%s", strings.TrimPrefix(deploymentName, "mcp-"))
	pods, err := p.client.CoreV1().Pods(p.namespace).List(
		context.TODO(),
		metav1.ListOptions{
			LabelSelector: labelSelector,
		},
	)
	if err != nil {
		return "", err
	}

	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no pods found for deployment %s", deploymentName)
	}

	// Return the first running pod
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodRunning {
			return pod.Name, nil
		}
	}

	// If no running pods, return the first pod
	return pods.Items[0].Name, nil
}

func (p *MCPProxy) waitForPodReady(podName string) error {
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for pod %s", podName)
		case <-ticker.C:
			pod, err := p.client.CoreV1().Pods(p.namespace).Get(
				context.TODO(),
				podName,
				metav1.GetOptions{},
			)
			if err != nil {
				continue
			}

			if pod.Status.Phase == corev1.PodRunning {
				// Check if container is ready
				for _, status := range pod.Status.ContainerStatuses {
					if status.Ready {
						return nil
					}
				}
			} else if pod.Status.Phase == corev1.PodFailed {
				return fmt.Errorf("pod %s failed: %s", podName, pod.Status.Message)
			}
		}
	}
}

func (p *MCPProxy) executeMCPRequest(podName string, serverName string, request *MCPRequest) (*MCPResponse, error) {
	// If this is a tools/call, strip the ms_ prefix from tool name
	originalRequest := *request // Keep a copy for logging
	if request.Method == "tools/call" {
		if params, ok := request.Params.(map[string]interface{}); ok {
			if toolName, ok := params["name"].(string); ok {
				// Strip ms_servername_ prefix to get original tool name
				if strings.HasPrefix(toolName, "ms_") {
					parts := strings.SplitN(toolName[3:], "_", 2) // Remove "ms_" and split on first "_"
					if len(parts) == 2 {
						params["name"] = parts[1] // Original tool name
					}
				}
			}
		}
	}

	// Convert request to JSON
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	// Get server config to build command
	serverConfig := p.getServerConfig(serverName)
	cmdStr := fmt.Sprintf("%s %s", serverConfig.Command, strings.Join(serverConfig.Args, " "))
	
	// Execute command in pod to send MCP request
	cmd := []string{"sh", "-c", fmt.Sprintf("echo '%s' | %s", string(requestJSON), cmdStr)}
	
	stdout, stderr, err := p.execInPod(podName, cmd)
	if err != nil {
		return nil, fmt.Errorf("exec error: %w, stderr: %s", err, stderr)
	}

	// Parse response
	var response MCPResponse
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w, output: %s", err, stdout)
	}

	// If this was a tools/list response, prefix tool names with ms_servername_
	if originalRequest.Method == "tools/list" && response.Result != nil {
		if result, ok := response.Result.(map[string]interface{}); ok {
			if tools, ok := result["tools"].([]interface{}); ok {
				cleanServerName := p.getServerName(serverName)
				for _, tool := range tools {
					if toolMap, ok := tool.(map[string]interface{}); ok {
						if name, ok := toolMap["name"].(string); ok {
							newName := fmt.Sprintf("ms_%s_%s", cleanServerName, name)
							toolMap["name"] = newName
						}
					}
				}
			}
		}
	}

	return &response, nil
}

// getServerName cleans server name keeping only alphanumeric and hyphens
func (p *MCPProxy) getServerName(name string) string {
	result := ""
	for _, char := range name {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || 
		   (char >= '0' && char <= '9') || char == '-' {
			result += string(char)
		}
	}
	return result
}

// extractServerName gets server name from prefixed tool name
func (p *MCPProxy) extractServerName(toolName string) string {
	if strings.HasPrefix(toolName, "ms_") {
		parts := strings.SplitN(toolName[3:], "_", 2) // Remove "ms_" and split on first "_"
		if len(parts) == 2 {
			return parts[0] // Server name
		}
	}
	return ""
}

func (p *MCPProxy) execInPod(podName string, cmd []string) (string, string, error) {
	req := p.client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(p.namespace).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Command: cmd,
		Stdin:   false,
		Stdout:  true,
		Stderr:  true,
		TTY:     false,
	}, scheme.ParameterCodec)

	config, err := (*p.config).ClientConfig()
	if err != nil {
		return "", "", err
	}

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return "", "", err
	}

	var stdout, stderr strings.Builder
	err = exec.StreamWithContext(context.TODO(), remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: &stdout,
		Stderr: &stderr,
	})

	return stdout.String(), stderr.String(), err
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}

func createKubernetesClient() (*kubernetes.Clientset, clientcmd.ClientConfig, error) {
	kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")
	config := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig},
		&clientcmd.ConfigOverrides{},
	)

	restConfig, err := config.ClientConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build config: %w", err)
	}

	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create client: %w", err)
	}

	return client, config, nil
}

func readMCPConfig(filename string) (*MCPConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var config MCPConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return &config, nil
}

func (p *MCPProxy) initializeMCPServerDeployments() error {
	fmt.Printf("🚀 Initializing MCP server deployments...\n")
	
	for _, server := range p.mcpConfig.MCPServers {
		deploymentName := fmt.Sprintf("mcp-%s", server.Name)
		
		// Check if deployment already exists
		_, err := p.client.AppsV1().Deployments(p.namespace).Get(
			context.TODO(),
			deploymentName,
			metav1.GetOptions{},
		)
		if err == nil {
			// Deployment already exists, skip creation
			log.Printf("✅ Deployment %s already exists, skipping creation", deploymentName)
			continue
		}
		
		log.Printf("🚀 Creating deployment for MCP server: %s", server.Name)
		deploymentName, err = p.createMCPServerDeployment(server.Name)
		if err != nil {
			return fmt.Errorf("failed to create deployment for server %s: %w", server.Name, err)
		}
		
		log.Printf("⏳ Waiting for deployment %s to be ready...", deploymentName)
		err = p.waitForDeploymentReady(deploymentName)
		if err != nil {
			return fmt.Errorf("deployment %s not ready: %w", deploymentName, err)
		}
		
		log.Printf("✅ Deployment %s is ready and running", deploymentName)
	}
	
	fmt.Printf("🎉 All MCP server deployments initialized successfully\n")
	return nil
}

func (p *MCPProxy) cleanupMCPServerDeployments() error {
	fmt.Printf("🚀 Cleaning up MCP server deployments...\n")
	
	for _, server := range p.mcpConfig.MCPServers {
		deploymentName := fmt.Sprintf("mcp-%s", server.Name)
		
		log.Printf("🚀 Deleting deployment for MCP server: %s", server.Name)
		err := p.deleteDeployment(deploymentName)
		if err != nil {
			return fmt.Errorf("failed to delete deployment for server %s: %w", server.Name, err)
		}
		
		log.Printf("⏳ Waiting for deployment %s to be deleted...", deploymentName)
		err = p.waitForDeploymentDeleted(deploymentName)
		if err != nil {
			return fmt.Errorf("deployment %s not deleted: %w", deploymentName, err)
		}
		
		log.Printf("✅ Deployment %s is deleted", deploymentName)
	}
	
	fmt.Printf("🎉 All MCP server deployments cleaned up successfully\n")
	return nil
} 