package provider

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"time"

	types "github.com/openeverest/provider-sdk/examples/psmdb/types"
	sdk "github.com/openeverest/provider-sdk/pkg/controller"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	psmdbv1 "github.com/percona/percona-server-mongodb-operator/pkg/apis/psmdb/v1"
)

const (
	SplitHorizonConfigMapKeyBaseDomainNameSuffix = "baseDomainNameSuffix"
	SplitHorizonConfigMapKeySecretName           = "secretName"
	CACertificateKey                             = "ca.crt"
	CAKeyKey                                     = "ca.key"
)

const (
	splitHorizonExternalKey = "external"
	publicIPPendingValue    = "pending"
)

var (
	errShardingNotSupported = errors.New("sharding is not supported for SplitHorizon DNS feature")
	validityNotAfter        = time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)
)

// getSpritHorizonFromCustomSpec retrieves the SplitHorizonDNSRef from the mongod component's CustomSpec.
// If SplitHorizonDNSRef is not set, it returns nil.
func getSpritHorizonFromCustomSpec(c *sdk.Context) (*types.SplitHorizonDNSRef, error) {
	engine, ok := c.DB().Spec.Components[ComponentEngine]
	if !ok {
		return nil, nil
	}

	if engine.CustomSpec == nil || engine.CustomSpec.Raw == nil {
		return nil, nil
	}

	mongodSpec := &types.MongodCustomSpec{}
	if err := c.DecodeComponentCustomSpec(engine, mongodSpec); err != nil {
		return nil, fmt.Errorf("failed to decode mongod CustomSpec: %w", err)
	}

	return mongodSpec.SplitHorizonDNSRef, nil
}

// getSplitHorizonConfig retrieves a pre-configured split horizon DNS configuration by reference.
// The configuration is expected to be stored as a ConfigMap in the specified or current namespace.
func getSplitHorizonConfig(c *sdk.Context, ref *types.SplitHorizonDNSRef) (*types.SplitHorizonDNSConfig, error) {
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
		if k8serrors.IsNotFound(err) {
			// ConfigMap not found, wait for it to be created
			return nil, sdk.WaitFor(fmt.Sprintf("split horizon configuration ConfigMap %s not found in namespace %s", configMapName, configNamespace))
		}
		return nil, fmt.Errorf("failed to retrieve split horizon configuration ConfigMap %s in namespace %s: %w", configMapName, configNamespace, err)
	}

	// Parse the configuration from the ConfigMap
	if configMap.Data == nil {
		return nil, fmt.Errorf("split horizon ConfigMap has no data")
	}

	baseDomain, ok := configMap.Data[SplitHorizonConfigMapKeyBaseDomainNameSuffix]
	if !ok || baseDomain == "" {
		return nil, fmt.Errorf("split horizon ConfigMap missing required key: baseDomainNameSuffix")
	}

	secretName, ok := configMap.Data[SplitHorizonConfigMapKeySecretName]
	if !ok || secretName == "" {
		return nil, fmt.Errorf("split horizon ConfigMap missing required key: secretName")
	}

	return &types.SplitHorizonDNSConfig{
		BaseDomainNameSuffix: baseDomain,
		SecretName:           secretName,
	}, nil
}

// configureSplitHorizon applies split horizon DNS configuration to PSMDB cluster spec.
// This configures the cluster for split horizon DNS setup, which allows clients to connect
// to MongoDB through different DNS names depending on the network/region they are in.
func configureSplitHorizon(c *sdk.Context, psmdb *psmdbv1.PerconaServerMongoDB, ref *types.SplitHorizonDNSRef) error { //nolint:funcorder
	shdc, err := getSplitHorizonConfig(c, ref)
	if err != nil {
		return err
	}

	spec := c.DB().Spec
	engine := spec.Components[ComponentEngine]

	if spec.Topology != nil && spec.Topology.Type == topologySharded {
		// For the time being SplitHorizon DNS feature can be applied for unsharded PSMDB clusters only.
		// Later we may consider adding support for sharded clusters as well.
		return errShardingNotSupported
	}

	shdcCaSecret := new(corev1.Secret)
	if err := c.Get(shdcCaSecret, shdc.SecretName); err != nil {
		return err
	}

	var nReplicas int32
	if engine.Replicas != nil {
		nReplicas = *engine.Replicas
	}

	horSpec := psmdbv1.HorizonsSpec{}
	for i := range nReplicas {
		horSpec[fmt.Sprintf("%s-rs0-%d", psmdb.GetName(), i)] = map[string]string{
			splitHorizonExternalKey: fmt.Sprintf("%s-rs0-%d-%s.%s",
				psmdb.GetName(),
				i,
				psmdb.GetNamespace(),
				shdc.BaseDomainNameSuffix),
		}
	}
	psmdb.Spec.Replsets[0].Horizons = horSpec

	needCreateSecret := false // do not generate server certificate on each reconciliation loop
	psmdbSplitHorizonSecretName := getSplitHorizonDNSConfigSecretName(psmdb.GetName())

	psmdbSplitHorizonSecret := &corev1.Secret{}
	if err := c.Get(psmdbSplitHorizonSecret, psmdbSplitHorizonSecretName); err != nil {
		if k8serrors.IsNotFound(err) {
			needCreateSecret = true
		} else {
			return err
		}
	}

	// TODO Should ConfigMap for Split Horizon DNS also hold certificates optionally just like CRD?
	if needCreateSecret {
		// Generate server TLS certificate for SplitHorizon DNS domains
		psmdbSplitHorizonDomains := []string{
			// Internal SANs
			"localhost",
			psmdb.GetName() + "-rs0",
			fmt.Sprintf("*.%s-rs0", psmdb.GetName()),
			fmt.Sprintf("%s-rs0.%s", psmdb.GetName(), psmdb.GetNamespace()),
			fmt.Sprintf("*.%s-rs0.%s", psmdb.GetName(), psmdb.GetNamespace()),
			fmt.Sprintf("%s-rs0.%s.svc.cluster.local", psmdb.GetName(), psmdb.GetNamespace()),
			fmt.Sprintf("*.%s-rs0.%s.svc.cluster.local", psmdb.GetName(), psmdb.GetNamespace()),
			// External SANs
			"*." + shdc.BaseDomainNameSuffix,
		}
		psmdbSplitHorizonServerCertBytes, psmdbSplitHorizonServerPrivKeyBytes, err := issueSplitHorizonCertificate(
			shdcCaSecret.Data["ca.crt"],
			shdcCaSecret.Data["ca.key"],
			psmdbSplitHorizonDomains)
		if err != nil {
			return fmt.Errorf("issue split-horizon server TLS certificate: %w", err)
		}

		// Store generated server TLS certificate in a secret. It is DB specific.
		psmdbSplitHorizonSecret.SetName(psmdbSplitHorizonSecretName)
		psmdbSplitHorizonSecret.SetNamespace(psmdb.GetNamespace())

		// TODO: use controllerutil.CreateOrUpdate to update only if data is changed
		psmdbSplitHorizonSecret.Data = map[string][]byte{
			"tls.crt": []byte(psmdbSplitHorizonServerCertBytes),
			"tls.key": []byte(psmdbSplitHorizonServerPrivKeyBytes),
			"ca.crt":  shdcCaSecret.Data["ca.crt"],
		}
		psmdbSplitHorizonSecret.Type = corev1.SecretTypeTLS

		if err := c.Apply(psmdbSplitHorizonSecret); err != nil {
			return fmt.Errorf("failed to create server TLS certificate secret: %w", err)
		}
	}

	// set reference to secret with certificate for external domain
	if psmdb.Spec.Secrets == nil {
		psmdb.Spec.Secrets = &psmdbv1.SecretsSpec{}
	}
	psmdb.Spec.Secrets.SSL = psmdbSplitHorizonSecret.GetName()

	return nil
}

func getSplitHorizonDNSConfigSecretName(dbName string) string {
	return dbName + "-sh-cert"
}

// statusSplitHorizon returns true is split horizon DNS service is ready.
// It returns true if split horizon DNS is not configured.
func statusSplitHorizon(c *sdk.Context, psmdb *psmdbv1.PerconaServerMongoDB) error {
	mongodSpec := &types.MongodCustomSpec{}
	if err := c.DecodeComponentCustomSpec(c.DB().Spec.Components[ComponentEngine], mongodSpec); err != nil {
		return fmt.Errorf("failed to decode mongod CustomSpec: %w", err)
	}

	if mongodSpec.SplitHorizonDNSRef == nil {
		// Nothing to do
		return nil
	}

	// Get generated external domains from PSMDB spec
	for podName := range psmdb.Spec.Replsets[0].Horizons {
		svc := &corev1.Service{}
		if err := c.Get(svc, podName); err != nil {
			if err = client.IgnoreNotFound(err); err != nil {
				return fmt.Errorf("failed to get service for SplitHorizon status: %w", err)
			}

			// service not created yet
			return fmt.Errorf("service %s not created yet", podName)
		}

		// TODO only apply for service type of corev1.ServiceTypeExternalName
		if _, ready := getServicePublicIP(svc); !ready {
			return fmt.Errorf("service %s not ready", podName)
		}
	}

	return nil
}

// getServicePublicIP retrieves the public IP address of the given service.
// It returns the public IP address and a boolean indicating whether the status is ready.
func getServicePublicIP(svc *corev1.Service) (string, bool) {
	if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return "", false
	}

	if len(svc.Status.LoadBalancer.Ingress) == 0 {
		return publicIPPendingValue, false
	}

	if svc.Status.LoadBalancer.Ingress[0].IP != "" {
		return svc.Status.LoadBalancer.Ingress[0].IP, true
	}

	// publicIP may be empty for load-balancer ingress points that are DNS based.
	// In such case Hostname shall be set (typically AWS)
	publicHostname := svc.Status.LoadBalancer.Ingress[0].Hostname
	if publicHostname == "" {
		return publicIPPendingValue, false
	}

	// try to resolve DNS name to IP
	var publicIPAddrs []net.IP
	var err error
	if publicIPAddrs, err = net.LookupIP(publicHostname); err != nil {
		// Binding IP to domain takes some time, so just log a warning here.
		fmt.Printf("resolve LoadBalancer ingress hostname %s to IP: %v\n", publicHostname, err)
		return publicIPPendingValue, false
	}

	for _, ip := range publicIPAddrs {
		if ipStr := ip.String(); ipStr != "<nil>" {
			return ipStr, true
		}
	}

	return publicIPPendingValue, false
}

// ----------------- Helpers -----------------
// issueSplitHorizonCertificate generates server TLS certificate signed by the provided CA certificate and private key,
// with SANs for the provided hosts.
// It returns the generated server TLS certificate, private key in PEM format and error.
func issueSplitHorizonCertificate(caCert, caPrivKey []byte, hosts []string) (string, string, error) {
	caDecoded, _ := pem.Decode(caCert)
	ca, err := x509.ParseCertificate(caDecoded.Bytes)
	if err != nil {
		return "", "", fmt.Errorf("parse CA certificate: %w", err)
	}

	caPrivKeyDecoded, _ := pem.Decode(caPrivKey)
	caKey, err := x509.ParsePKCS1PrivateKey(caPrivKeyDecoded.Bytes)
	if err != nil {
		return "", "", fmt.Errorf("parse CA private key: %w", err)
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128) //nolint:mnd
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return "", "", fmt.Errorf("generate serial number for client: %w", err)
	}

	serverCertTemplate := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"PSMDB"},
		},
		NotBefore:             time.Now(),
		NotAfter:              validityNotAfter,
		IPAddresses:           []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback}, //nolint:mnd
		DNSNames:              hosts,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
	}
	// Create server certificate private key
	certPrivKey, err := rsa.GenerateKey(rand.Reader, 2048) //nolint:mnd
	if err != nil {
		return "", "", fmt.Errorf("generate client key: %w", err)
	}

	// Create and sign server certificate with CA
	serverCertBytes, err := x509.CreateCertificate(rand.Reader, &serverCertTemplate, ca, &certPrivKey.PublicKey, caKey)
	if err != nil {
		return "", "", fmt.Errorf("generate server certificate: %w", err)
	}
	serverCertPem := &bytes.Buffer{}
	err = pem.Encode(serverCertPem, &pem.Block{Type: "CERTIFICATE", Bytes: serverCertBytes})
	if err != nil {
		return "", "", fmt.Errorf("encode server certificate: %w", err)
	}

	serverKeyPem := &bytes.Buffer{}
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(certPrivKey)}
	err = pem.Encode(serverKeyPem, block)
	if err != nil {
		return "", "", fmt.Errorf("encode RSA private key: %w", err)
	}

	return serverCertPem.String(), serverKeyPem.String(), nil
}
