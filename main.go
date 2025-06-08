package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type MCPConfig struct {
	MCPServers []MCPServer `yaml:"mcp-servers"`
}

type MCPServer struct {
	Name        string            `yaml:"name"`
	Image       string            `yaml:"image"`
	Command     string            `yaml:"command"`
	Args        []string          `yaml:"args"`
	Env         map[string]string `yaml:"env,omitempty"`
	Interactive bool              `yaml:"interactive,omitempty"`
}

type MCPController struct {
	client    *kubernetes.Clientset
	namespace string
}

func main() {
	fmt.Println("🚀 Starting MCP Kubernetes Controller...")

	// Initialize Kubernetes client
	client, err := createKubernetesClient()
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	controller := &MCPController{
		client:    client,
		namespace: "default",
	}

	fmt.Println("✅ Connected to Kubernetes cluster")
	
	// Start the controller loop
	controller.Run()
}

func (c *MCPController) Run() {
	fmt.Println("🔄 Controller started - watching for changes...")
	
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	// Initial reconciliation
	c.reconcile()

	// Continuous reconciliation loop
	for {
		select {
		case <-ticker.C:
			c.reconcile()
		}
	}
}

func (c *MCPController) reconcile() {
	fmt.Printf("[%s] 🔍 Reconciling MCP deployments...\n", time.Now().Format("15:04:05"))

	// Read current mcp.yaml configuration
	mcpConfig, err := readMCPConfig("mcp.yaml")
	if err != nil {
		log.Printf("❌ Failed to read MCP config: %v", err)
		return
	}

	// Get current deployments
	currentDeployments, err := c.getCurrentMCPDeployments()
	if err != nil {
		log.Printf("❌ Failed to get current deployments: %v", err)
		return
	}

	// Track desired deployments
	desiredDeployments := make(map[string]MCPServer)
	for _, server := range mcpConfig.MCPServers {
		desiredDeployments[fmt.Sprintf("mcp-%s", server.Name)] = server
	}

	// Create or update deployments for desired servers
	for deploymentName, server := range desiredDeployments {
		if _, exists := currentDeployments[deploymentName]; exists {
			// Always update to ensure desired state
			fmt.Printf("📝 Updating deployment: %s\n", deploymentName)
			err := c.forceUpdateMCPDeployment(server)
			if err != nil {
				log.Printf("❌ Failed to update deployment %s: %v", deploymentName, err)
			} else {
				fmt.Printf("✅ Updated deployment: %s\n", deploymentName)
			}
		} else {
			// Create new deployment
			fmt.Printf("🆕 Creating deployment: %s\n", deploymentName)
			err := c.createMCPDeployment(server)
			if err != nil {
				log.Printf("❌ Failed to create deployment %s: %v", deploymentName, err)
			} else {
				fmt.Printf("✅ Created deployment: %s\n", deploymentName)
			}
		}
	}

	// Delete deployments that are no longer desired
	for deploymentName := range currentDeployments {
		if _, desired := desiredDeployments[deploymentName]; !desired {
			fmt.Printf("🗑️  Deleting deployment: %s\n", deploymentName)
			err := c.deleteMCPDeployment(deploymentName)
			if err != nil {
				log.Printf("❌ Failed to delete deployment %s: %v", deploymentName, err)
			} else {
				fmt.Printf("✅ Deleted deployment: %s\n", deploymentName)
			}
		}
	}

	// Get deployment status
	c.printDeploymentStatus()
}

func (c *MCPController) getCurrentMCPDeployments() (map[string]*appsv1.Deployment, error) {
	deployments, err := c.client.AppsV1().Deployments(c.namespace).List(
		context.TODO(),
		metav1.ListOptions{
			LabelSelector: "mcp-server",
		},
	)
	if err != nil {
		return nil, err
	}

	result := make(map[string]*appsv1.Deployment)
	for i := range deployments.Items {
		deployment := &deployments.Items[i]
		result[deployment.Name] = deployment
	}
	return result, nil
}

func (c *MCPController) needsUpdate(deployment *appsv1.Deployment, server MCPServer) bool {
	// Check if the image needs updating based on command type
	container := deployment.Spec.Template.Spec.Containers[0]
	expectedImage := c.getExpectedImage(server)
	
	if container.Image != expectedImage {
		return true
	}

	// Check interactive mode settings
	if container.Stdin != server.Interactive || container.TTY != server.Interactive {
		return true
	}

	// Check environment variables
	expectedEnvVars := c.getExpectedEnvVars(server)
	if len(container.Env) != len(expectedEnvVars) {
		return true
	}

	envMap := make(map[string]string)
	for _, env := range container.Env {
		envMap[env.Name] = env.Value
	}

	for _, expectedEnv := range expectedEnvVars {
		if envMap[expectedEnv.Name] != expectedEnv.Value {
			return true
		}
	}

	return false
}

func (c *MCPController) getExpectedImage(server MCPServer) string {
	if server.Image != "" {
		return server.Image
	}
	return "alpine:latest"
}

func (c *MCPController) getExpectedEnvVars(server MCPServer) []corev1.EnvVar {
	var envVars []corev1.EnvVar
	if server.Env != nil {
		for key, value := range server.Env {
			envVars = append(envVars, corev1.EnvVar{
				Name:  key,
				Value: value,
			})
		}
	}
	return envVars
}

func (c *MCPController) deleteMCPDeployment(deploymentName string) error {
	return c.client.AppsV1().Deployments(c.namespace).Delete(
		context.TODO(),
		deploymentName,
		metav1.DeleteOptions{},
	)
}

func (c *MCPController) printDeploymentStatus() {
	deployments, err := c.getCurrentMCPDeployments()
	if err != nil {
		log.Printf("❌ Failed to get deployment status: %v", err)
		return
	}

	fmt.Printf("📊 Current MCP Deployments Status:\n")
	for name, deployment := range deployments {
		ready := deployment.Status.ReadyReplicas
		desired := *deployment.Spec.Replicas
		fmt.Printf("   • %s: %d/%d ready\n", name, ready, desired)
	}
	fmt.Println()
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

func createKubernetesClient() (*kubernetes.Clientset, error) {
	kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build config: %w", err)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return client, nil
}

func (c *MCPController) createMCPDeployment(server MCPServer) error {
	deploymentName := fmt.Sprintf("mcp-%s", server.Name)

	// Create container spec based on command type
	container := createContainerSpec(server)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: c.namespace,
			Labels: map[string]string{
				"app":        deploymentName,
				"mcp-server": server.Name,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": deploymentName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":        deploymentName,
						"mcp-server": server.Name,
					},
				},
				Spec: corev1.PodSpec{
					Containers:    []corev1.Container{container},
					RestartPolicy: corev1.RestartPolicyAlways,
				},
			},
		},
	}

	_, err := c.client.AppsV1().Deployments(c.namespace).Create(
		context.TODO(),
		deployment,
		metav1.CreateOptions{},
	)

	return err
}

func (c *MCPController) updateMCPDeployment(server MCPServer) error {
	deploymentName := fmt.Sprintf("mcp-%s", server.Name)

	// Get current deployment
	deployment, err := c.client.AppsV1().Deployments(c.namespace).Get(
		context.TODO(),
		deploymentName,
		metav1.GetOptions{},
	)
	if err != nil {
		return err
	}

	// Update the container spec
	container := createContainerSpec(server)
	deployment.Spec.Template.Spec.Containers[0] = container

	_, err = c.client.AppsV1().Deployments(c.namespace).Update(
		context.TODO(),
		deployment,
		metav1.UpdateOptions{},
	)

	return err
}

func (c *MCPController) forceUpdateMCPDeployment(server MCPServer) error {
	deploymentName := fmt.Sprintf("mcp-%s", server.Name)

	// Get current deployment
	deployment, err := c.client.AppsV1().Deployments(c.namespace).Get(
		context.TODO(),
		deploymentName,
		metav1.GetOptions{},
	)
	if err != nil {
		return err
	}

	// Update the container spec and force rolling update with annotation
	container := createContainerSpec(server)
	deployment.Spec.Template.Spec.Containers[0] = container
	
	// Add restart annotation to force rolling update
	if deployment.Spec.Template.Annotations == nil {
		deployment.Spec.Template.Annotations = make(map[string]string)
	}
	deployment.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

	_, err = c.client.AppsV1().Deployments(c.namespace).Update(
		context.TODO(),
		deployment,
		metav1.UpdateOptions{},
	)

	return err
}

func createContainerSpec(server MCPServer) corev1.Container {
	container := corev1.Container{
		Name: server.Name,
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceMemory: mustParseQuantity("128Mi"),
				corev1.ResourceCPU:    mustParseQuantity("100m"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: mustParseQuantity("512Mi"),
				corev1.ResourceCPU:    mustParseQuantity("500m"),
			},
		},
		// Configure interactive mode for stdio communication
		Stdin: server.Interactive,
		TTY:   server.Interactive,
		// MCP servers don't expose HTTP endpoints, so we'll use a simple command-based liveness probe
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				Exec: &corev1.ExecAction{
					Command: []string{"sh", "-c", "ps aux | grep -q '[0-9]' || exit 1"},
				},
			},
			InitialDelaySeconds: 30,
			PeriodSeconds:       10,
			FailureThreshold:    3,
		},
	}

	// Set the image and command
	container.Image = server.Image
	if server.Image == "" {
		container.Image = "alpine:latest"
	}
	
	// Configure command and args
	if server.Command != "" {
		container.Command = []string{server.Command}
		container.Args = server.Args
	}

	// Add environment variables from config
	if server.Env != nil {
		for key, value := range server.Env {
			container.Env = append(container.Env, corev1.EnvVar{
				Name:  key,
				Value: value,
			})
		}
	}

	return container
}

func int32Ptr(i int32) *int32 {
	return &i
}

func mustParseQuantity(s string) resource.Quantity {
	q, _ := resource.ParseQuantity(s)
	return q
}