package runtime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nsxbet/mcpshield/pkg"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
)

type KubernetesRuntime struct {
	client         *kubernetes.Clientset
	config         *KubernetesConfig
	namespace      string
	deploymentName string
	image          string
	command        string
	args           []string
	env            map[string]string
}

type KubernetesConfig struct {
	ClientConfig clientcmd.ClientConfig
}

type KubernetesRuntimeFactory struct {
	client    *kubernetes.Clientset
	config    *KubernetesConfig
	namespace string
}

func NewKubernetesRuntimeFactory(namespace string) (pkg.RuntimeFactory, error) {
	client, clientConfig, err := CreateKubernetesClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	return &KubernetesRuntimeFactory{
		client:    client,
		config:    &KubernetesConfig{ClientConfig: clientConfig},
		namespace: namespace,
	}, nil
}

func NewKubernetesRuntimeFactoryWithClient(client *kubernetes.Clientset, clientConfig clientcmd.ClientConfig, namespace string) pkg.RuntimeFactory {
	return &KubernetesRuntimeFactory{
		client:    client,
		config:    &KubernetesConfig{ClientConfig: clientConfig},
		namespace: namespace,
	}
}

func (f *KubernetesRuntimeFactory) CreateRuntime(image, command string, args []string, env map[string]string) pkg.Runtime {
	return &KubernetesRuntime{
		client:    f.client,
		config:    f.config,
		namespace: f.namespace,
		image:     image,
		command:   command,
		args:      args,
		env:       env,
	}
}

func (k *KubernetesRuntime) Start(ctx context.Context) error {
	k.deploymentName = fmt.Sprintf("mcp-%s", k.getCleanName())
	
	// Check if deployment already exists and delete it
	_, err := k.client.AppsV1().Deployments(k.namespace).Get(ctx, k.deploymentName, metav1.GetOptions{})
	if err == nil {
		// Deployment exists, delete it first
		if err := k.deleteDeployment(ctx); err != nil {
			return fmt.Errorf("failed to delete existing deployment: %w", err)
		}
		// Wait for deletion to complete
		if err := k.waitForDeploymentDeleted(ctx); err != nil {
			return fmt.Errorf("failed to wait for deployment deletion: %w", err)
		}
	}
	
	if err := k.createDeployment(ctx); err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}
	
	if err := k.waitForDeploymentReady(ctx); err != nil {
		k.deleteDeployment(context.Background())
		return fmt.Errorf("deployment not ready: %w", err)
	}
	
	return nil
}

func (k *KubernetesRuntime) Exec(ctx context.Context, input []byte) ([]byte, error) {
	podName, err := k.getPodFromDeployment()
	if err != nil {
		return nil, fmt.Errorf("failed to get pod: %w", err)
	}
	
	if err := k.waitForPodReady(ctx, podName); err != nil {
		return nil, fmt.Errorf("pod not ready: %w", err)
	}
	
	cmdStr := fmt.Sprintf("%s %s", k.command, strings.Join(k.args, " "))
	cmd := []string{"sh", "-c", fmt.Sprintf("echo '%s' | %s", string(input), cmdStr)}
	
	stdout, stderr, err := k.execInPod(ctx, podName, cmd)
	if err != nil {
		return nil, fmt.Errorf("exec error: %w, stderr: %s", err, stderr)
	}
	
	return []byte(stdout), nil
}

func (k *KubernetesRuntime) Stop() error {
	if k.deploymentName == "" {
		return nil
	}
	
	ctx := context.Background()
	if err := k.deleteDeployment(ctx); err != nil {
		return err
	}
	
	// Wait for deployment to be fully deleted
	return k.waitForDeploymentDeleted(ctx)
}

func (k *KubernetesRuntime) IsReady() bool {
	if k.deploymentName == "" {
		return false
	}
	
	deployment, err := k.client.AppsV1().Deployments(k.namespace).Get(
		context.Background(),
		k.deploymentName,
		metav1.GetOptions{},
	)
	if err != nil {
		return false
	}
	
	return deployment.Status.ReadyReplicas >= *deployment.Spec.Replicas
}

func (k *KubernetesRuntime) createDeployment(ctx context.Context) error {
	replicas := int32(1)
	
	envVars := []corev1.EnvVar{}
	for key, value := range k.env {
		expandedValue := os.ExpandEnv(value)
		envVars = append(envVars, corev1.EnvVar{
			Name:  key,
			Value: expandedValue,
		})
	}
	
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k.deploymentName,
			Namespace: k.namespace,
			Labels: map[string]string{
				"app":     "mcp-bridge",
				"runtime": "kubernetes",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":        "mcp-bridge",
					"deployment": k.deploymentName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":        "mcp-bridge",
						"deployment": k.deploymentName,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyAlways,
					Containers: []corev1.Container{
						{
							Name:    "mcp-server",
							Image:   k.image,
							Command: []string{k.command},
							Args:    k.args,
							Env:     envVars,
							Stdin:   true,
							TTY:     true,
						},
					},
				},
			},
		},
	}
	
	_, err := k.client.AppsV1().Deployments(k.namespace).Create(
		ctx,
		deployment,
		metav1.CreateOptions{},
	)
	return err
}

func (k *KubernetesRuntime) waitForDeploymentReady(ctx context.Context) error {
	timeout := time.After(60 * time.Second)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for deployment %s", k.deploymentName)
		case <-ticker.C:
			if k.IsReady() {
				return nil
			}
		}
	}
}

func (k *KubernetesRuntime) deleteDeployment(ctx context.Context) error {
	propagationPolicy := metav1.DeletePropagationForeground
	return k.client.AppsV1().Deployments(k.namespace).Delete(
		ctx,
		k.deploymentName,
		metav1.DeleteOptions{
			PropagationPolicy: &propagationPolicy,
		},
	)
}

func (k *KubernetesRuntime) waitForDeploymentDeleted(ctx context.Context) error {
	timeout := time.After(60 * time.Second)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for deployment %s to be deleted", k.deploymentName)
		case <-ticker.C:
			_, err := k.client.AppsV1().Deployments(k.namespace).Get(ctx, k.deploymentName, metav1.GetOptions{})
			if err != nil {
				// Deployment not found, deletion successful
				return nil
			}
		}
	}
}

func (k *KubernetesRuntime) getPodFromDeployment() (string, error) {
	labelSelector := fmt.Sprintf("app=mcp-bridge,deployment=%s", k.deploymentName)
	pods, err := k.client.CoreV1().Pods(k.namespace).List(
		context.Background(),
		metav1.ListOptions{
			LabelSelector: labelSelector,
		},
	)
	if err != nil {
		return "", err
	}
	
	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no pods found for deployment %s", k.deploymentName)
	}
	
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodRunning {
			return pod.Name, nil
		}
	}
	
	return pods.Items[0].Name, nil
}

func (k *KubernetesRuntime) waitForPodReady(ctx context.Context, podName string) error {
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for pod %s", podName)
		case <-ticker.C:
			pod, err := k.client.CoreV1().Pods(k.namespace).Get(
				context.Background(),
				podName,
				metav1.GetOptions{},
			)
			if err != nil {
				continue
			}
			
			if pod.Status.Phase == corev1.PodRunning {
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

func (k *KubernetesRuntime) execInPod(ctx context.Context, podName string, cmd []string) (string, string, error) {
	req := k.client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(k.namespace).
		SubResource("exec")
	
	req.VersionedParams(&corev1.PodExecOptions{
		Command: cmd,
		Stdin:   false,
		Stdout:  true,
		Stderr:  true,
		TTY:     false,
	}, scheme.ParameterCodec)
	
	config, err := k.getRESTConfig()
	if err != nil {
		return "", "", err
	}
	
	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return "", "", err
	}
	
	var stdout, stderr strings.Builder
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: &stdout,
		Stderr: &stderr,
	})
	
	return stdout.String(), stderr.String(), err
}

func (k *KubernetesRuntime) getRESTConfig() (*rest.Config, error) {
	return k.config.ClientConfig.ClientConfig()
}

func (k *KubernetesRuntime) getCleanName() string {
	// Clean the image name to create a valid deployment name
	name := strings.ToLower(k.image)
	name = strings.ReplaceAll(name, ":", "-")
	name = strings.ReplaceAll(name, "/", "-")
	if len(name) > 40 {
		name = name[:40]
	}
	return name
}

func CreateKubernetesClient() (*kubernetes.Clientset, clientcmd.ClientConfig, error) {
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