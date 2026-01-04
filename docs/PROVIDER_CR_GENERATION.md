# Provider CR Generation Guide

This document explains how to generate the Provider CR manifest that describes your provider's component types, versions, and topologies.

## Overview

Every provider needs a `Provider` CR (Custom Resource) that tells Everest:
- What component types are available (e.g., `mongod`, `postgres`)
- What versions are supported for each type
- What logical components use those types (e.g., `engine`, `proxy`)
- What topologies are supported (e.g., `replicaset`, `sharded`)

The Provider CR is generated from metadata you define in Go code and included in your Helm chart.

## Generation Flow

```
┌─────────────────┐    go run    ┌─────────────────┐    git commit    ┌─────────────────┐
│   Go Metadata   │ ───────────►  │  provider.yaml  │ ──────────────►  │   Helm Chart    │
│   (source)      │  generate-    │   (manifest)    │                  │   (deployed)    │
│                 │  manifest     │                 │                  │                 │
└─────────────────┘               └─────────────────┘                  └─────────────────┘
```

## Step 1: Define Metadata in Go

First, define your provider's metadata using the SDK types:

```go
// metadata.go
package main

import sdk "github.com/openeverest/provider-sdk/pkg/controller"

func PSMDBMetadata() *sdk.ProviderMetadata {
    return &sdk.ProviderMetadata{
        // Component types define versions and images
        ComponentTypes: map[string]sdk.ComponentTypeMeta{
            "mongod": {
                Versions: []sdk.ComponentVersionMeta{
                    {
                        Version: "6.0.19-16",
                        Image:   "percona/percona-server-mongodb:6.0.19-16",
                    },
                    {
                        Version: "8.0.8-3",
                        Image:   "percona/percona-server-mongodb:8.0.8-3",
                        Default: true,  // Mark default version
                    },
                },
            },
            "backup": {
                Versions: []sdk.ComponentVersionMeta{
                    {
                        Version: "2.5.0",
                        Image:   "percona/percona-backup-mongodb:2.5.0",
                        Default: true,
                    },
                },
            },
        },
        
        // Components map logical names to types
        Components: map[string]sdk.ComponentMeta{
            "engine":       {Type: "mongod"},
            "configServer": {Type: "mongod"},
            "backupAgent":  {Type: "backup"},
        },
        
        // Topologies define valid deployment configurations
        Topologies: map[string]sdk.TopologyMeta{
            "replicaset": {
                Components: map[string]sdk.TopologyComponentMeta{
                    "engine":      {Optional: false},  // Required
                    "backupAgent": {Optional: true},   // Optional
                },
            },
            "sharded": {
                Components: map[string]sdk.TopologyComponentMeta{
                    "engine":       {Optional: false},
                    "configServer": {Optional: false},
                    "backupAgent":  {Optional: true},
                },
            },
        },
    }
}
```

## Step 2: Create Generation Tool

Create a CLI tool to generate the manifest:

```go
// cmd/generate-manifest/main.go
package main

import (
    "flag"
    "log"
    "os"
    
    sdk "github.com/openeverest/provider-sdk/pkg/controller"
)

func main() {
    output := flag.String("output", "charts/provider.yaml", "Output file path")
    name := flag.String("name", "psmdb", "Provider name")
    namespace := flag.String("namespace", "", "Namespace (empty for cluster-scoped)")
    flag.Parse()
    
    // Get your provider metadata
    metadata := PSMDBMetadata()
    
    // Generate the YAML
    yaml, err := sdk.GenerateManifest(metadata, *name, *namespace, *output)
    if err != nil {
        log.Fatalf("Failed to generate manifest: %v", err)
    }
    
    log.Printf("Generated Provider CR at %s", *output)
}
```

## Step 3: Add to Build Process

Add the generation step to your Makefile:

```makefile
# Makefile

# Generate the Provider CR manifest
.PHONY: generate-provider
generate-provider:
	@echo "Generating Provider CR manifest..."
	go run ./cmd/generate-manifest --output charts/provider.yaml --name psmdb

# Make sure it runs before building
.PHONY: build
build: generate-provider
	docker build -t my-provider:latest .

# Add to your CI/CD verification
.PHONY: verify
verify: generate-provider
	@git diff --exit-code charts/provider.yaml || \
		(echo "Error: provider.yaml is out of sync. Run 'make generate-provider'" && exit 1)
```

## Step 4: Include in Helm Chart

Add the generated manifest to your Helm chart:

```yaml
# charts/templates/provider.yaml
{{ .Files.Get "provider.yaml" }}
```

Or if you want to make it configurable:

```yaml
# charts/templates/provider.yaml
{{- if .Values.provider.install }}
{{ .Files.Get "provider.yaml" }}
{{- end }}
```

**Production Deployment:** Your provider Helm chart should also include the underlying database operator as a dependency or bundled installation. For example, a PSMDB provider chart should install the Percona Server MongoDB Operator along with the Provider CR. This ensures all required components are deployed together.

## Step 5: Commit the Generated File

The generated `provider.yaml` should be committed to Git:

```bash
# Generate the file
make generate-provider

# Review the changes
git diff charts/provider.yaml

# Commit it
git add charts/provider.yaml
git commit -m "Update Provider CR with new versions"
```

## Generated Output Example

The tool generates a complete Provider CR like this:

```yaml
apiVersion: everest.percona.com/v2alpha1
kind: Provider
metadata:
  name: psmdb
spec:
  componentTypes:
    mongod:
      versions:
      - version: "6.0.19-16"
        image: "percona/percona-server-mongodb:6.0.19-16"
        default: false
      - version: "8.0.8-3"
        image: "percona/percona-server-mongodb:8.0.8-3"
        default: true
    backup:
      versions:
      - version: "2.5.0"
        image: "percona/percona-backup-mongodb:2.5.0"
        default: true
  components:
    engine:
      type: mongod
    configServer:
      type: mongod
    backupAgent:
      type: backup
  topologies:
    replicaset:
      components:
        engine:
          optional: false
        backupAgent:
          optional: true
    sharded:
      components:
        engine:
          optional: false
        configServer:
          optional: false
        backupAgent:
          optional: true
```

## Best Practices

### 1. Keep Metadata in Sync

Your provider code should use the same metadata:

```go
func NewPSMDBProvider() *PSMDBProvider {
    return &PSMDBProvider{
        BaseProvider: sdk.BaseProvider{
            ProviderName: "psmdb",
            Metadata:     PSMDBMetadata(),  // Same metadata!
        },
    }
}
```

This ensures consistency and allows helper functions like `c.Metadata()` to work.

### 2. Verify in CI/CD

Add a check to ensure the manifest is always up-to-date:

```yaml
# .github/workflows/ci.yml
- name: Verify Provider CR is up-to-date
  run: |
    make generate-provider
    git diff --exit-code charts/provider.yaml
```

### 3. Version Your Images

Use specific image tags, not `latest`:

```go
{
    Version: "8.0.8-3",
    Image:   "percona/percona-server-mongodb:8.0.8-3",  // ✓ Good
}

// Not this:
{
    Version: "latest",
    Image:   "percona/percona-server-mongodb:latest",  // ✗ Bad
}
```

### 4. Mark Default Versions Explicitly

```go
Versions: []sdk.ComponentVersionMeta{
    {Version: "6.0.19-16", Image: "...", Default: false},
    {Version: "8.0.8-3", Image: "...", Default: true},  // Clear default
}
```

### 5. Document Breaking Changes

When updating topologies or component types, document the changes:

```go
// v2.0.0: Removed "monitoring" component from replicaset topology
// v2.0.0: Added "proxy" component to sharded topology
```

### 6. Bundle Database Operator in Production

Your provider Helm chart should include the underlying database operator:

```yaml
# Chart.yaml
dependencies:
  - name: percona-server-mongodb-operator
    version: "1.21.1"
    repository: "https://percona.github.io/percona-helm-charts"
```

Or include the operator manifests directly in your chart templates. This ensures the operator is installed automatically with your provider, rather than requiring manual installation.

## Troubleshooting

### Manifest Not Updating

```bash
# Force regeneration
rm charts/provider.yaml
make generate-provider
```

### Invalid YAML

The generator validates metadata before creating YAML. Check for:
- Duplicate default versions
- Invalid component type references
- Missing required fields

### Helm Installation Fails

Ensure the Provider CRD is installed first:

```bash
kubectl apply -f config/crd/bases/everest.percona.com_providers.yaml
```

## Related Documentation

- [SDK Overview](../SDK_OVERVIEW.md) - Architecture and concepts
- [Metadata Helpers](../METADATA_HELPERS.md) - Using metadata in your provider code
- [Examples](../../examples/README.md) - Complete PSMDB implementation
- [metadata.go](../../pkg/controller/metadata.go) - Metadata types reference

