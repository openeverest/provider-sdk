# Provider Development Guide

This guide walks you through everything you need to define and implement to create
a working OpenEverest provider. It covers the conceptual model, the definition
directory structure, the provider implementation, and common patterns.

> **Reference implementation**: See
> [provider-percona-server-mongodb](https://github.com/openeverest/provider-percona-server-mongodb)
> for a complete, working example of a provider for Percona Server for MongoDB.

## Table of Contents

- [Conceptual Model](#conceptual-model)
- [Project Structure Overview](#project-structure-overview)
- [Step 1: Define Components](#step-1-define-components)
- [Step 2: Define Versions](#step-2-define-versions)
  - [Component Version Catalog](#component-version-catalog)
  - [Version Bundles](#version-bundles)
- [Step 3: Define Topologies](#step-3-define-topologies)
- [Step 4: Define Custom Types](#step-4-define-custom-types)
- [Step 5: Configure the UI Schema](#step-5-configure-the-ui-schema)
- [Step 6: Implement the Provider Interface](#step-6-implement-the-provider-interface)
- [Step 7: Configure RBAC](#step-7-configure-rbac)
- [Step 8: Generate and Test](#step-8-generate-and-test)
- [Provider SDK CLI Reference](#provider-sdk-cli-reference)

---

## Conceptual Model

An OpenEverest **Provider** bridges the gap between the platform's generic
**Instance** abstraction and a specific upstream database operator. The provider
defines *what* can be deployed (components, versions, topologies) and *how* to
translate Instance specs into upstream operator resources.

### Key Concepts

| Concept | Description | Example (MongoDB) |
|---------|-------------|-------------------|
| **Component** | A logical part of a database deployment | `engine`, `proxy`, `backupAgent`, `monitoring` |
| **Component Type** | The software a component runs | `mongod`, `backup`, `pmm` |
| **Topology** | A deployment architecture combining components | `replicaSet`, `sharded` |
| **Version** | A specific release of a component type | `8.0.12-4` (mongod), `2.11.0` (backup) |
| **Provider Interface** | The Go interface you implement | `Validate`, `Sync`, `Status`, `Cleanup` |

### How It Fits Together

```
User creates an Instance CR
        │
        ▼
Provider Runtime receives the Instance
        │
        ▼
Your Provider implementation:
  1. Validate() - validates the Instance spec
  2. Sync()     - creates/updates upstream operator CRs
  3. Status()   - reads upstream status → Instance status
  4. Cleanup()  - deletes upstream resources on Instance deletion
```

### Component Names vs Component Types

This distinction is important:

- **Component names** are logical roles within your provider (e.g., `engine`,
  `proxy`, `configServer`). Multiple components can share the same type.
- **Component types** define what software runs (e.g., `mongod`). Types have
  version catalogs with container images.

For example, in a sharded MongoDB deployment, both `engine` and `configServer`
components use the `mongod` type — they run the same software but serve
different roles.

---

## Project Structure Overview

```
definition/                          # ← YOU EDIT THESE
  provider.yaml                      # Provider name + component→type mapping
  versions.yaml                      # Component type version/image catalog
  types.go                           # Shared Go types (TopologyType, GlobalConfig)
  components/
    types.go                         # Component custom spec types (CustomSpec structs)
  topologies/
    <topology-name>/
      topology.yaml                  # Topology config + UI schema
      types.go                       # Topology-specific config types

internal/                            # ← YOU IMPLEMENT THESE
  provider/
    provider.go                      # ProviderInterface methods (Validate/Sync/Status/Cleanup)
    rbac.go                          # Kubebuilder RBAC markers
  common/
    spec.go                          # Component name/type constants

charts/<provider-name>/              # ← GENERATED (mostly)
  generated/
    provider-spec.yaml               # Generated from definition/ by `provider-sdk generate`
    rbac-rules.yaml                  # Generated from rbac.go by `make manifests`
  templates/                         # Helm chart templates (edit if needed)
```

---

## Step 1: Define Components

Components are the building blocks of your provider. Each component represents
a logical part of the database deployment.

### Using the CLI

```bash
# The init command creates one component ("engine") automatically.
# Add more components:
provider-sdk add component --name backupAgent --type backup
provider-sdk add component --name monitoring --type pmm
provider-sdk add component --name proxy --type mongod
provider-sdk add component --name configServer --type mongod
```

### Manual Editing

Edit `definition/provider.yaml`:

```yaml
name: my-provider

components:
  engine:
    type: mongod
  configServer:
    type: mongod          # Same type as engine — different role
  proxy:
    type: mongod
  backupAgent:
    type: backup
  monitoring:
    type: pmm
```

### What Gets Updated

When you add a component (via CLI or manually), these files need to be in sync:

| File | What to add |
|------|-------------|
| `definition/provider.yaml` | Component entry under `components:` |
| `definition/versions.yaml` | Component type entry if new type |
| `definition/components/types.go` | `CustomSpec` struct if component needs custom config |
| `internal/common/spec.go` | Constants for component name and type |

The `provider-sdk add component` command updates all four files automatically.

---

## Step 2: Define Versions

All version information lives in `definition/versions.yaml`. It has two
related sections: the **component version catalog** and **version bundles**.

### Component Version Catalog

The catalog maps each component type to the list of versions it supports.
This is the source of truth for what can be installed.

```yaml
componentTypes:
  mongod:
    versions:
    - version: "8.0.12-4"
      image: percona/percona-server-mongodb:8.0.12-4
      default: true                    # Exactly one version must be default
    - version: "7.0.18-11"
      image: percona/percona-server-mongodb:7.0.18-11
    - version: "6.0.21-18"
      image: percona/percona-server-mongodb:6.0.21-18
  backup:
    versions:
    - version: "2.11.0"
      image: percona/percona-backup-mongodb:2.11.0
      default: true
  pmm:
    versions:
    - version: "2.44.1"
      image: percona/pmm-server:2.44.1
      default: true
```

**Rules:**
- Each component type must have at least one version
- Exactly one version per type must be marked `default: true`
- Images should be fully qualified (registry/repository:tag)
- Add new versions when upstream releases are available

### Version Bundles

Version bundles are curated sets of component versions that are known to be
mutually compatible. Users set a single `spec.version` field on an Instance
instead of specifying versions for every component individually.

Bundles are defined in the same `definition/versions.yaml` file, under a
top-level `versions:` key:

```yaml
versions:
- name: "8.0.12"                       # Bundle name — shown to users
  default: true                        # Used when spec.version is omitted
  components:
    engine: "8.0.12-4"
    configServer: "8.0.12-4"
    proxy: "8.0.12-4"
    backupAgent: "2.11.0"
    monitoring: "2.44.1"
- name: "8.0.8"
  components:
    engine: "8.0.8-3"
    configServer: "8.0.8-3"
    proxy: "8.0.8-3"
    backupAgent: "2.9.1"
    monitoring: "2.44.1"
```

**How bundles are resolved**

The provider-runtime resolves bundles in the reconciler before calling your
`Sync()` method. Your `Sync()` code always sees fully-resolved component
versions and does not need to handle bundle logic itself.

Resolution order for each component's version:

1. `ComponentSpec.Version` — explicitly set by the user on that component (wins)
2. Version bundle — from `spec.version` or the default bundle
3. Per-type `default: true` in the catalog — fallback if no bundle applies

The reconciler operates on a **deep copy** of the Instance. The stored spec in
etcd is never mutated, so the user's original intent is always preserved.

**Rules:**
- Exactly one bundle should have `default: true`
- Every component name and version referenced in a bundle must exist in
  `provider.yaml` and in the corresponding `componentTypes` catalog
  respectively — `provider-sdk generate` validates this at build time
- Bundle names are arbitrary strings but should follow a human-friendly
  convention (e.g., the upstream operator's minor version)
- You do not need to include optional components (e.g., `monitoring`) in
  a bundle; the user can still specify their version explicitly

**Adding a new bundle when a new upstream version is released**

1. Add the new component versions to `componentTypes` in `versions.yaml`
2. Add a new bundle entry under `versions:` referencing those new versions
3. Move `default: true` to the new bundle
4. Run `make generate` — the generator validates all bundle references and
   emits the updated `Provider` CR spec

**Accessing bundle info in your provider code**

You normally do not need to interact with bundles directly in `Sync()` —
versions are already resolved for you. However, if you need to inspect the
resolved version of a component, read it from the Instance spec as usual:

```go
func (p *Provider) Sync(c *controller.Context) error {
    engine := c.Instance().Spec.Components["engine"]
    // engine.Version is already resolved from the bundle (e.g., "8.0.12-4")
    // engine.Image is empty unless the user explicitly overrode it

    spec, err := c.ProviderSpec()
    if err != nil {
        return err
    }
    // Resolve the image from the catalog using the resolved version:
    image := controller.GetImageForVersion(spec, "engine", engine.Version)
    // Or fall back to the default image if Version is somehow still empty:
    image = controller.GetDefaultImageForComponent(spec, "engine")
    // ...
}
```

---

## Step 3: Define Topologies

Topologies define deployment architectures — which components are used together
and how they're configured.

### Using the CLI

```bash
# The init command creates one topology automatically.
# Add more topologies:
provider-sdk add topology --name replicaSet
provider-sdk add topology --name sharded
```

### Topology YAML Structure

Each topology lives in `definition/topologies/<name>/topology.yaml`:

```yaml
# config section: defines the topology structure
config:
  # Optional: reference a Go type for custom topology config
  configSchema: ShardedTopologyConfig

  # List which components this topology uses
  components:
    engine:
      defaults:
        replicas: 3                    # Default value for this topology
    proxy: {}                          # Required, no defaults
    configServer: {}                   # Required, no defaults
    backupAgent:
      optional: true                   # User can choose to enable/disable
    monitoring:
      optional: true

# ui section: rendering hints for the frontend form
ui:
  sections:
    basicInfo:
      label: Basic Information
      description: Provide the basic information for your new database.
      components:
        version:
          uiType: select
          path: spec.components.engine.version
          fieldParams:
            label: Database Version
    resources:
      label: Resources
      description: Configure the resources for your cluster.
      components:
        numberOfNodes:
          path: spec.components.engine.replicas
          uiType: number
          fieldParams:
            label: Number of nodes per shard
          validation:
            min: 1
            max: 7
  sectionsOrder:
  - basicInfo
  - resources
```

### Component Options in Topologies

| Field | Description |
|-------|-------------|
| `defaults.replicas` | Default replica count for this topology |
| `optional: true` | Component can be enabled/disabled by the user |
| `{}` | Required component with no special defaults |

### Topology Config Types

If a topology needs custom configuration beyond component specs (e.g., number
of shards), create a Go type:

```go
// In definition/topologies/sharded/types.go
package sharded

type ShardedTopologyConfig struct {
    NumShards int32 `json:"numShards,omitempty"`
}
```

Then reference it in topology.yaml:

```yaml
config:
  configSchema: ShardedTopologyConfig
```

The `provider-sdk generate` command converts the Go struct to an OpenAPI schema
and embeds it in the Provider CR.

**Accessing topology config in your provider:**

```go
func (p *Provider) Sync(c *controller.Context) error {
    var cfg sharded.ShardedTopologyConfig
    if c.TryDecodeTopologyConfig(&cfg) {
        numShards := cfg.NumShards
        // Use the topology config...
    }
    // ...
}
```

---

## Step 4: Define Custom Types

Custom types allow you to extend the Instance spec with provider-specific fields.

### Component Custom Specs

If a component needs fields beyond the standard `replicas`, `resources`, `storage`,
and `version`, define a `CustomSpec` struct:

```go
// In definition/components/types.go
package components

type MongodCustomSpec struct {
    // WiredTigerCacheSizeGB sets the WiredTiger cache size.
    WiredTigerCacheSizeGB float64 `json:"wiredTigerCacheSizeGB,omitempty"`
}
```

Then reference it in `provider.yaml`:

```yaml
components:
  engine:
    type: mongod
    customSpecSchema: MongodCustomSpec
```

### Shared Types

Define provider-wide types in `definition/types.go`:

```go
package definition

type TopologyType string

const (
    TopologyTypeReplicaSet TopologyType = "replicaSet"
    TopologyTypeSharded    TopologyType = "sharded"
)

type GlobalConfig struct{}
```

---

## Step 5: Configure the UI Schema

The UI schema in each `topology.yaml` tells the OpenEverest frontend how to
render the Instance creation/edit form.

### Sections

Sections are logical groupings (typically form steps). Each section has:

| Property | Type | Description |
|----------|------|-------------|
| `label` | string | Display name |
| `description` | string | Section description |
| `components` | object | Component or ComponentGroup definitions |
| `componentsOrder` | array | Optional display order of components |

```yaml
resources:
  label: Basic Info
  description: Provider the basic information for your DB.
  components:
    version: { ... }
  componentsOrder:
    - version
```

Sections render as steps in a multi-step form:

```
  ① Basic Info       ② Resources        ③ Advanced
  ─────●─────────────────○─────────────────○──────
 ┌──────────────────────────────────────────────┐
 │ Basic Info                                   │
 │ Provide the basic information for your DB.   │
 │                                              │
 │  ┌────────────────────────────────────────┐  │
 │  │ Database Version       [8.0.12     ▾]  │  │
 │  └────────────────────────────────────────┘  │
 │                                              │
 │                              [ Next → ]      │
 └──────────────────────────────────────────────┘
```

### UI Types

Each component has a `uiType` that determines how it renders.

| `uiType` | Description |
|-----------|-------------|
| `number` | Numeric input |
| `select` | Dropdown selector |
| `text` | Text input |
| `hidden` | Not displayed, value excluded from form and API payload |
| `group` | Groups components |

### Components (Single Fields)

A **Component** is a single form field:

| Property | Type | Description |
|----------|------|-------------|
| `uiType` | string | `number`, `select`, `text`, `hidden` |
| `path` | string | Dot-notation path in Instance spec (e.g. `spec.components.engine.replicas`) |
| `fieldParams` | object | Field configuration (see each type below) |
| `validation` | object | Validation rules (see [Validation](#validation-1)) |

#### Number Field

```
┌──────────────────────────────────┐
│ Number of nodes                  │
│ ┌────────────────────────┐       │
│ │ 3                    ↕ │       │
│ └────────────────────────┘       │
└──────────────────────────────────┘
```

```yaml
numberOfNodes:
  uiType: number
  path: spec.components.engine.replicas
  fieldParams:
    label: Number of nodes
    defaultValue: 3
    step: 1
    modes:
      edit:
        disabled: true
  validation:
    min: 1
    max: 7
```

Supported `fieldParams`:

| Param | Type | Description |
|-------|------|-------------|
| `label` | string | Field label |
| `defaultValue` | number | Initial value |
| `step` | number | Increment/decrement step (e.g. `0.1`) |
| `badge` | string | Unit label shown next to the input (e.g. `"Gi"`, `"cores"`) |
| `badgeToApi` | bool | When `true`, appends `badge` to the value sent to the API (e.g. `4` → `"4Gi"`) |
| `disabled` | bool | Field is non-interactive |
| `modes` | object | Per-mode overrides (see [Mode-Aware Overrides](#mode-aware-overrides)) |

#### Select Field

```
┌──────────────────────────────────┐
│ Database Version                 │
│ ┌──────────────────────────┐     │
│ │ 8.0.12                 ▾ │     │
│ └──────────────────────────┘     │
└──────────────────────────────────┘
```

```yaml
version:
  uiType: select
  path: spec.engine.version
  fieldParams:
    label: Database Version
    optionsPath: spec.versions
    optionsPathConfig:
      labelPath: "name"
      valuePath: "name"
    modes:
      edit:
        disabled: true
  validation:
    required: true
```

Supported `fieldParams`:

| Param | Type | Description |
|-------|------|-------------|
| `label` | string | Field label |
| `defaultValue` | string | Pre-selected value |
| `options` | array | Inline options: `[{ label, value }]` |
| `optionsPath` | string | Path in the Provider spec to load options from |
| `optionsPathConfig` | object | Maps object fields to label/value: `{ labelPath, valuePath }` |
| `displayEmpty` | bool | Adds an empty "None" option for optional fields |
| `disabled` | bool | Field is non-interactive |
| `modes` | object | Per-mode overrides (see [Mode-Aware Overrides](#mode-aware-overrides)) |

#### Text Field

```
┌──────────────────────────────────┐
│ Configuration                    │
│ ┌────────────────────────────┐   │
│ │ operationProfiling:        │   │
│ │   mode: slowOp             │   │
│ │                            │   │
│ └────────────────────────────┘   │
└──────────────────────────────────┘
```

```yaml
configuration:
  uiType: text
  path: spec.components.engine.configuration
  fieldParams:
    label: Configuration
    placeholder: |
      operationProfiling:
        mode: slowOps
    multiline: true
    minRows: 3
    maxRows: 8
```

Supported `fieldParams`:

| Param | Type | Description |
|-------|------|-------------|
| `label` | string | Field label |
| `defaultValue` | string | Initial value |
| `placeholder` | string | Placeholder text shown when empty |
| `multiline` | bool | Renders a textarea instead of a single-line input |
| `minRows` | number | Minimum visible rows (when `multiline: true`) |
| `maxRows` | number | Maximum visible rows before scrolling |
| `disabled` | bool | Field is non-interactive |
| `modes` | object | Per-mode overrides (see [Mode-Aware Overrides](#mode-aware-overrides)) |

#### Hidden Field

```yaml
internalId:
  uiType: hidden
  path: spec.internalId
```

Not rendered. Value excluded from the form and API payload.

### ComponentGroups (Nested Fields)

Set `uiType: group` to group multiple components together.

Supported properties:

| Property | Type | Description |
|----------|------|-------------|
| `uiType` | string | Must be `group` |
| `groupType` | string | `line` (horizontal layout) or `accordion` (collapsible) |
| `label` | string | Group label (displayed differently per `groupType`) |
| `description` | string | Group description |
| `components` | object | Nested components or groups |
| `componentsOrder` | array | Optional display order |

**Line group** — renders children horizontally:

```yaml
resources:
  label: "Resources"
  uiType: group
  groupType: line
  components:
    cpu:
      uiType: number
      path: spec.components.engine.resources.limits.cpu
      fieldParams:
        label: CPU
        defaultValue: 1
    memory:
      uiType: number
      path: spec.components.engine.resources.limits.memory
      fieldParams:
        label: Memory
        defaultValue: 4
        badge: "Gi"
        badgeToApi: true
  componentsOrder:
    - cpu
    - memory
```

```
┌─ Resources ─────────────────────────────┐
│  ┌──────────────┐  ┌──────────────┐     │
│  │ CPU          │  │ Memory       │     │
│  │ ┌────────┐   │  │ ┌────────┐   │     │
│  │ │ 1    ↕ │   │  │ │ 4    ↕ │Gi │     │
│  │ └────────┘   │  │ └────────┘   │     │
│  └──────────────┘  └──────────────┘     │
└─────────────────────────────────────────┘
```

**Accordion group** — renders as a collapsible panel:

```
┌─ ▶ Advanced Settings ───────────────────────┐
└─────────────────────────────────────────────┘

┌─ ▼ Advanced Settings ───────────────────────┐
│                                             │
│  Storage class                              │
│  ┌───────────────────────────────────┐      │
│  │ local-path                      ▾ │      │
│  └───────────────────────────────────┘      │
│                                             │
│  Configuration                              │
│  ┌───────────────────────────────────┐      │
│  │ operationProfiling:               │      │
│  │   mode: slowOp                    │      │
│  └───────────────────────────────────┘      │
└─────────────────────────────────────────────┘
```

### Path and ID References

Each component must have either `path` or `id` (not both):

- **`path`** — dot-notation location in the Instance spec. The value is included in the API payload.
- **`id`** — custom identifier for fields that should **not** be submitted but are needed for validation or conditional rendering.

```yaml
# path — value is stored in the Instance spec
nodes:
  uiType: number
  path: spec.components.engine.replicas
  fieldParams:
    label: Number of nodes

# id — value is NOT submitted, used only for UI logic
confirmDeletion:
  uiType: text
  id: confirmDeletion
  fieldParams:
    label: Type the database name to confirm
  validation:
    celExpressions:
      - celExpr: "confirmDeletion == metadata.name"
        message: Name does not match
```

Common `path` examples:

```
spec.components.<name>.resources.limits.cpu  → CPU limit
spec.components.<name>.storage.size          → Storage size
```

### Validation

Add a `validation` object to any component. All rules are optional and
composable:

| Rule | Type | Description |
|------|------|-------------|
| `required` | bool | Field must have a value |
| `min` / `max` | number | Numeric bounds |
| `int` | bool | Must be an integer |
| `regex` | object | `pattern` + optional `message` |
| `celExpressions` | array | Cross-field rules using CEL |

```yaml
validation:
  required: true
  min: 1
  int: true
  celExpressions:
    - celExpr: "spec.components.engine.replicas % 2 == 1"
      message: "The number of nodes must be odd"
```

CEL expressions can reference any field by its `path` and `id`.
In edit mode, `original.<path>` gives the persisted value for
comparison.

### Mode-Aware Overrides

Override component behavior per mode (`new`, `edit`, `restore`, `import`).

**fieldParams-level** — presentation overrides:

```yaml
fieldParams:
  label: Database name
  modes:
    edit:
      disabled: true
```

**validation-level** — per-mode validation rules:

```yaml
validation:
  required: true
  min: 1
  modes:
    edit:
      celExpressions:
        - celExpr: "spec.sharding.shards >= original.spec.sharding.shards"
          message: Number of shards cannot be decreased
```

Mode-specific scalar rules **replace** base rules. Mode-specific
`celExpressions` are **appended** to base expressions.

### DataSource (API-Driven Select Options)

Use `dataSource` instead of inline `options` when a select field's choices come
from the OpenEverest API at runtime:

```yaml
storageClass:
  uiType: select
  dataSource:
    provider: storageClasses
  path: spec.engine.storage.class
  fieldParams:
    label: Storage class
```

Available providers: `storageClasses`, `monitoringConfigs`. The field
auto-selects the first option, shows loading/error states, and renders an
empty-state fallback when the provider supports one. Everything else
(`validation`, `modes`, `fieldParams`) works identically to a regular select.

### Example

The YAML below produces a form that renders like this:

```
 ── Step 1: Database Version ──────────────────────────
 Provide the information about the database version
 you want to use.
┌────────────────────────────────────────────────────┐
│ Database Version                                   │
│ ┌────────────────────────────────────────────────┐ │
│ │ 8.0.12                                       ▾ │ │
│ └────────────────────────────────────────────────┘ │
└────────────────────────────────────────────────────┘

 ── Step 2: Resources ─────────────────────────────────
 Configure the resources your new database will
 have access to.
┌────────────────────────────────────────────────────┐
│ Number of nodes                                    │
│ ┌────────────┐                                     │
│ │ 3        ↕ │                                     │
│ └────────────┘                                     │
│                                                    │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐    │
│  │ CPU        │  │ Memory     │  │ Disk       │    │
│  │ ┌──────┐   │  │ ┌──────┐   │  │ ┌──────┐   │    │
│  │ │ 1  ↕ │   │  │ │ 4  ↕ │Gi │  │ │ 25 ↕ │Gi │    │
│  │ └──────┘   │  │ └──────┘   │  │ └──────┘   │    │
│  └────────────┘  └────────────┘  └────────────┘    │
└────────────────────────────────────────────────────┘

 ── Step 3: Advanced configuration ────────────────────
 Configure advanced settings for your database.
┌────────────────────────────────────────────────────┐
│ Storage class                                      │
│ ┌────────────────────────────────────────────────┐ │
│ │ local-path                                   ▾ │ │
│ └────────────────────────────────────────────────┘ │
│                                                    │
│ Engine configuration                               │
│ ┌────────────────────────────────────────────────┐ │
│ │ operationProfiling:                            │ │
│ │   mode: slowOps                                │ │
│ │                                                │ │
│ └────────────────────────────────────────────────┘ │
└────────────────────────────────────────────────────┘
```

```yaml
# definition/topologies/replicaSet/topology.yaml
ui:
  sections:
    databaseVersion:
      label: "Database Version"
      description: "Provide the information about the database version you want to use."
      components:
        version:
          uiType: select
          path: spec.version
          fieldParams:
            label: "Database Version"
            optionsPath: spec.versions
            optionsPathConfig:
              labelPath: "name"
              valuePath: "name"
            modes:
              edit:
                disabled: true
          validation:
            required: true
    resources:
      label: "Resources"
      description: "Configure the resources your new database will have access to."
      components:
        nodes:
          uiType: group
          components:
            numberOfnodes:
              path: "spec.components.engine.replicas"
              uiType: number
              fieldParams:
                label: "Number of nodes"
                defaultValue: 3
              validation:
                required: true
                min: 1
                int: true
                celExpressions:
                  - celExpr: "spec.components.engine.replicas % 2 == 1"
                    message: "The number of nodes must be odd"
                modes:
                  edit:
                    celExpressions:
                      - celExpr: "!(spec.components.engine.replicas == 1 && original.spec.components.engine.replicas > 1)"
                        message: "Cannot scale down to a single node"
            resources:
              uiType: group
              groupType: line
              components:
                cpu:
                  path: "spec.components.engine.resources.limits.cpu"
                  uiType: number
                  fieldParams:
                    label: "CPU"
                    defaultValue: 1
                    step: 1
                  validation:
                    min: 0.6
                    required: true
                memory:
                  path: "spec.components.engine.resources.limits.memory"
                  uiType: number
                  fieldParams:
                    label: "Memory"
                    defaultValue: 4
                    step: 0.1
                    badge: "Gi"
                    badgeToApi: true
                  validation:
                    min: 0.512
                    required: true
                disk:
                  path: "spec.components.engine.storage.size"
                  uiType: number
                  fieldParams:
                    label: "Disk"
                    defaultValue: 25
                    badge: "Gi"
                    badgeToApi: true
                  validation:
                    min: 1
                    int: true
                    required: true
                    modes:
                      edit:
                        celExpressions:
                          - celExpr: "spec.components.engine.storage.size >= original.spec.components.engine.storage.size"
                            message: "Disk size cannot be decreased"
          componentsOrder:
            - numberOfnodes
            - resources
    advanced:
      label: "Advanced configuration"
      description: "Configure advanced settings for your database"
      components:
        storageClass:
          uiType: select
          path: spec.components.engine.storage.storageClass
          fieldParams:
            label: "Storage class"
            modes:
              edit:
                disabled: true
          validation:
            required: true
          dataSource:
            provider: storageClasses
        configuration:
          uiType: text
          path: spec.components.engine.configuration
          fieldParams:
            label: "Engine configuration"
            placeholder: |2
              operationProfiling:
                mode: slowOps
                slowOpThresholdMs: 200
            multiline: true
            minRows: 3
            maxRows: 8
      componentsOrder:
        - storageClass
        - configuration
  sectionsOrder:
    - databaseVersion
    - resources
    - advanced
```

---

## Step 6: Implement the Provider Interface

The core of your provider is in `internal/provider/provider.go`. You must
implement four methods:

### Validate

Called before Sync. Validate the Instance spec and return an error for
invalid configurations.

```go
func (p *Provider) Validate(c *controller.Context) error {
    spec := c.Instance().Spec

    engine := spec.Components["engine"]
    if engine.Replicas != nil && *engine.Replicas < 1 {
        return fmt.Errorf("engine replicas must be at least 1")
    }

    if spec.Topology != nil && spec.Topology.Type == "sharded" {
        if _, ok := spec.Components["proxy"]; !ok {
            return fmt.Errorf("sharded topology requires a proxy component")
        }
    }

    return nil
}
```

### Sync

Create or update upstream operator resources. This is called on every
reconciliation.

```go
func (p *Provider) Sync(c *controller.Context) error {
    // Build the upstream CR from the Instance spec
    upstream := &operatorv1.MyDatabase{
        ObjectMeta: c.ObjectMeta(c.Name()),  // Sets ownership
        Spec:       buildUpstreamSpec(c),
    }

    // Apply creates or updates the resource
    if err := c.Apply(upstream); err != nil {
        return err
    }

    return nil
}
```

### Status

Read the upstream resource status and translate it to an Instance status.

```go
func (p *Provider) Status(c *controller.Context) (controller.Status, error) {
    upstream := &operatorv1.MyDatabase{}
    if err := c.Get(upstream, c.Name()); err != nil {
        return controller.Provisioning("Waiting for upstream resource"), nil
    }

    switch upstream.Status.State {
    case "ready":
        details, err := buildConnectionDetails(c, psmdb)
		    if err != nil {
			    return controller.Failed("Failed to build connection details: " + err.Error()), nil
        }

		    return controller.ReadyWithConnectionDetails(details), nil
    case "error":
        return controller.Failed(upstream.Status.Message), nil
    default:
        return controller.Provisioning("Cluster is being created"), nil
    }
}
```

### Cleanup

Handle deletion. Delete upstream resources when the Instance is deleted.

```go
func (p *Provider) Cleanup(c *controller.Context) error {
    upstream := &operatorv1.MyDatabase{
        ObjectMeta: c.ObjectMeta(c.Name()),
    }
    return c.Delete(upstream)
}
```

### Provider Setup

Configure your provider with schemes and watch configs:

```go
func New() *Provider {
    return &Provider{
        BaseProvider: controller.BaseProvider{
            ProviderName: common.ProviderName,
            SchemeFuncs: []func(*runtime.Scheme) error{
                operatorv1.SchemeBuilder.AddToScheme,  // Register upstream types
            },
            WatchConfigs: []controller.WatchConfig{
                controller.WatchOwned(&operatorv1.MyDatabase{}),  // Watch upstream CRs
            },
        },
    }
}
```

### Helper Patterns

**Getting component specs:**

```go
engine := c.Instance().Spec.Components["engine"]
// engine.Replicas, engine.Resources, engine.Storage, engine.Version, engine.Image
```

**Getting default images from the Provider spec:**

```go
spec, err := c.ProviderSpec()
if err != nil {
    return err
}
image := controller.GetDefaultImageForComponent(spec, "engine")
```

**Decoding topology config:**

```go
var cfg sharded.ShardedTopologyConfig
if c.TryDecodeTopologyConfig(&cfg) {
    // Use cfg.NumShards, etc.
}
```

---

## Step 7: Configure RBAC

RBAC permissions are declared using
[kubebuilder markers](https://book.kubebuilder.io/reference/markers/rbac)
in `internal/provider/rbac.go`.

### Base RBAC (pre-configured)

The scaffold includes base RBAC for the provider runtime:
- Instances, Providers (read/update)
- Leases (leader election)
- Events (recording)

### Adding Provider-Specific RBAC

Add markers for your upstream operator's resources:

```go
// Upstream operator primary resource
// +kubebuilder:rbac:groups=psmdb.percona.com,resources=perconaservermongodbs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=psmdb.percona.com,resources=perconaservermongodbs/status,verbs=get
// +kubebuilder:rbac:groups=psmdb.percona.com,resources=perconaservermongodbs/finalizers,verbs=update

// Core Kubernetes resources your provider manages
// +kubebuilder:rbac:groups="",resources=secrets;configmaps;services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
```

**Common resources to consider:**
- Upstream operator CRDs (main resource, backups, restores)
- Secrets (connection strings, credentials)
- ConfigMaps (operator config)
- Services (database endpoints)
- PersistentVolumeClaims (storage)
- Pods (monitoring, status)

After adding markers, run `make generate` to regenerate RBAC manifests.

---

## Step 8: Generate and Test

### Code Generation

```bash
# Generate everything: RBAC manifests, provider spec, Helm chart files
make generate
```

This runs:
1. `controller-gen` → `config/rbac/role.yaml` (from kubebuilder markers)
2. `helm-sync-rbac` → `charts/.../generated/rbac-rules.yaml`
3. `go generate` → `charts/.../generated/provider-spec.yaml` (from definition/)

### Local Testing

```bash
# Create a local cluster
make k3d-cluster-up

# Install prerequisites (OpenEverest CRDs, upstream operator)
kubectl apply -f https://raw.githubusercontent.com/openeverest/openeverest/v2/config/crd/bases/core.openeverest.io_providers.yaml
kubectl apply -f https://raw.githubusercontent.com/openeverest/openeverest/v2/config/crd/bases/core.openeverest.io_instances.yaml
# Install your upstream operator...

# Deploy the provider with Helm
make helm-install

# Or run locally for development
make run

# Apply a test instance
kubectl apply -f examples/instance-example.yaml

# Check status
kubectl get instances
kubectl get providers
```

### Integration Tests

```bash
# Run kuttl integration tests
make test-integration
```

Edit test files in `test/integration/` to add test cases for your provider.

### CI Verification

```bash
# Verify generated files are up-to-date
make verify
```

---

## Provider SDK CLI Reference

### `provider-sdk init`

Scaffold a new provider project.

```bash
provider-sdk init \
  --name provider-my-database \
  --module github.com/my-org/provider-my-database \
  --component-type mydb \
  --topology standalone
```

### `provider-sdk add component`

Add a component to an existing provider project.

```bash
provider-sdk add component --name backupAgent --type backup
```

Updates: `provider.yaml`, `versions.yaml`, `components/types.go`, `spec.go`

### `provider-sdk add topology`

Add a topology to an existing provider project.

```bash
provider-sdk add topology --name replicaSet
```

Creates: `topologies/<name>/topology.yaml`, `topologies/<name>/types.go`

### `provider-sdk generate`

Generate the Provider CR spec from definition/ files.

```bash
provider-sdk generate
```

Usually invoked via `go generate ./...` (see `gen.go`) or `make generate`.

The generator also validates version bundles at build time — it ensures every
component name and version referenced in a bundle exists in `provider.yaml` and
the `componentTypes` catalog. An invalid bundle produces a fatal error rather
than silently emitting a broken Provider CR.

---

## Checklist: What You Need for a Working Provider

Use this checklist to track your progress:

- [ ] **Components defined** in `definition/provider.yaml`
- [ ] **Version catalog** filled in `definition/versions.yaml`
- [ ] **Version bundles** defined in `definition/versions.yaml` with one marked `default: true`
- [ ] **At least one topology** in `definition/topologies/`
- [ ] **UI schema** configured in each topology's `topology.yaml`
- [ ] **Provider interface** implemented in `internal/provider/provider.go`:
  - [ ] `Validate()` — validates Instance spec
  - [ ] `Sync()` — creates/updates upstream resources
  - [ ] `Status()` — translates upstream status
  - [ ] `Cleanup()` — handles deletion
- [ ] **Upstream operator scheme** registered in `SchemeFuncs`
- [ ] **Watch configs** set for upstream resources
- [ ] **RBAC markers** added for all upstream resources
- [ ] **Component constants** in `internal/common/spec.go`
- [ ] **Custom types** (if needed) in `definition/components/types.go`
- [ ] **Topology config types** (if needed) in `definition/topologies/*/types.go`
- [ ] **`make generate`** runs without errors
- [ ] **Integration tests** pass
