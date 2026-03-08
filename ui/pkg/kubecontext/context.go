package kubecontext

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

// Manager handles kubeconfig context switching.
type Manager struct {
	kubeconfig string
}

// NewManager creates a kubecontext manager.
func NewManager() *Manager {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home, _ := os.UserHomeDir()
		kubeconfig = filepath.Join(home, ".kube", "config")
	}
	return &Manager{kubeconfig: kubeconfig}
}

// GetContexts returns available kubeconfig contexts.
func (m *Manager) GetContexts() ([]string, error) {
	config, err := m.loadConfig()
	if err != nil {
		return nil, err
	}

	contexts := make([]string, 0, len(config.Contexts))
	for name := range config.Contexts {
		contexts = append(contexts, name)
	}
	return contexts, nil
}

// GetCurrentContext returns the active kubeconfig context.
func (m *Manager) GetCurrentContext() (string, error) {
	config, err := m.loadConfig()
	if err != nil {
		return "", err
	}
	return config.CurrentContext, nil
}

// GetRestConfig returns a rest.Config for the given context (or current if empty).
func (m *Manager) GetRestConfig(contextName string) (*rest.Config, error) {
	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: m.kubeconfig}
	overrides := &clientcmd.ConfigOverrides{}
	if contextName != "" {
		overrides.CurrentContext = contextName
	}
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides).ClientConfig()
}

// SwitchContext changes the current context in the kubeconfig file.
func (m *Manager) SwitchContext(contextName string) error {
	config, err := m.loadConfig()
	if err != nil {
		return err
	}

	if _, ok := config.Contexts[contextName]; !ok {
		return fmt.Errorf("context %q not found", contextName)
	}

	config.CurrentContext = contextName
	return clientcmd.WriteToFile(*config, m.kubeconfig)
}

func (m *Manager) loadConfig() (*api.Config, error) {
	return clientcmd.LoadFromFile(m.kubeconfig)
}
