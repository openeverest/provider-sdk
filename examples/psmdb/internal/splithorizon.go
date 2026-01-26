package provider

import (
	"fmt"

	types "github.com/openeverest/provider-sdk/examples/psmdb/types"
	sdk "github.com/openeverest/provider-sdk/pkg/controller"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
)

const (
	SplitHorizonConfigMapKeyBaseDomainNameSuffix = "baseDomainNameSuffix"
	SplitHorizonConfigMapKeySecretName           = "secretName"
	CACertificateKey                             = "ca.crt"
	CAKeyKey                                     = "ca.key"
)

// GetSplitHorizonConfigByRef retrieves a pre-configured split horizon DNS configuration by reference.
// The configuration is expected to be stored as a ConfigMap in the specified or current namespace.
func GetSplitHorizonConfigByRef(c *sdk.Context, ref *types.SplitHorizonDNSRef) (*types.SplitHorizonDNSSpec, error) {
	// Determine the namespace to look in
	configNamespace := ref.Namespace
	if configNamespace == "" {
		// Use the DataStore's namespace
		configNamespace = c.Namespace()
	}

	// Retrieve the ConfigMap
	configMapName := ref.Name
	configMap := &corev1.ConfigMap{}

	if err := c.Get(configMap, configMapName); err != nil {
		if errors.IsNotFound(err) {
			// ConfigMap not found, wait for it to be created
			return nil, sdk.WaitFor(fmt.Sprintf("split horizon configuration ConfigMap %s not found in namespace %s", configMapName, configNamespace))
		}
		return nil, fmt.Errorf("failed to retrieve split horizon configuration ConfigMap %s in namespace %s: %w", configMapName, configNamespace, err)
	}

	// Parse the configuration from the ConfigMap
	cm, err := parseSplitHorizonConfigFromConfigMap(configMap)
	if err != nil {
		return nil, fmt.Errorf("failed to parse split horizon configuration from ConfigMap %s in namespace %s: %w", configMapName, configNamespace, err)
	}

	secret := new(corev1.Secret)
	if err := c.Get(secret, cm.SecretName); err != nil {
		if errors.IsNotFound(err) {
			// Secret not found, wait for it to be created
			return nil, sdk.WaitFor(fmt.Sprintf("split horizon TLS Secret %s not found in namespace %s", cm.SecretName, configNamespace))
		}
		return nil, fmt.Errorf("failed to retrieve split horizon TLS Secret %s in namespace %s: %w", cm.SecretName, configNamespace, err)
	}

	sec, err := parseSplitHorizonCerts(secret)
	if err != nil {
		return nil, fmt.Errorf("failed to parse split horizon TLS Secret %s in namespace %s: %w", cm.SecretName, configNamespace, err)
	}

	return &types.SplitHorizonDNSSpec{
		Config:      *cm,
		Certificate: *sec,
	}, nil
}

// parseSplitHorizonConfigFromConfigMap extracts split horizon configuration from a ConfigMap.
func parseSplitHorizonConfigFromConfigMap(cm *corev1.ConfigMap) (*types.SplitHorizonDNSConfig, error) {
	if cm.Data == nil {
		return nil, fmt.Errorf("split horizon ConfigMap has no data")
	}

	baseDomain, ok := cm.Data[SplitHorizonConfigMapKeyBaseDomainNameSuffix]
	if !ok || baseDomain == "" {
		return nil, fmt.Errorf("split horizon ConfigMap missing required key: baseDomainNameSuffix")
	}

	secretName, ok := cm.Data[SplitHorizonConfigMapKeySecretName]
	if !ok || secretName == "" {
		return nil, fmt.Errorf("split horizon ConfigMap missing required key: secretName")
	}

	return &types.SplitHorizonDNSConfig{
		BaseDomainNameSuffix: baseDomain,
		SecretName:           secretName,
	}, nil
}

// parseSplitHorizonCerts extracts split horizon certificates from a Secret.
func parseSplitHorizonCerts(secret *corev1.Secret) (*types.SplitHorizonDNSConfigTLSCertificateSpec, error) {
	if secret.Data == nil {
		return nil, fmt.Errorf("split horizon ConfigMap has no data")
	}

	cert, ok := secret.Data[CACertificateKey]
	if !ok || cert == nil {
		return nil, fmt.Errorf("split horizon Secret missing required key: ca.crt")
	}

	key, ok := secret.Data[CAKeyKey]
	if !ok || key == nil {
		return nil, fmt.Errorf("split horizon Secret missing required key: ca.key")
	}

	return &types.SplitHorizonDNSConfigTLSCertificateSpec{
		CACert: string(cert),
		CAKey:  string(key),
	}, nil
}
