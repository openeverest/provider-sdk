# Tutorial: build your first provider

By the end of this you will have a provider running in a local cluster, an `Instance` you
created through it, and a clear idea of which file to open when you want to change something.

It takes about an hour. You need Go, Docker, kubectl, Helm and [k3d](https://k3d.io) — no
OpenEverest checkout, and no database operator.

The provider you build here is [provider-example](https://github.com/openeverest/provider-example):
it runs **memcached** on a StatefulSet, with no operator in between. That is deliberate. A
provider backed by a real operator forces you to learn that operator's API before a single
OpenEverest concept comes into view; removing it leaves the provider contract as the only
thing on screen. Everything you learn here applies unchanged to a provider that drives an
operator — only the body of `Sync` and `Status` differs.

Clone it and read along, or type it out yourself:

```bash
git clone https://github.com/openeverest/provider-example
```

For the full detail on anything below, see the
[provider development guide](PROVIDER_DEVELOPMENT.md). This tutorial takes one path through
the material and skips the alternatives on purpose.

**Contents**

1. [What a provider is](#1-what-a-provider-is)
2. [Scaffold the project](#2-scaffold-the-project)
3. [Declare what you deploy](#3-declare-what-you-deploy)
4. [Make it real: Sync](#4-make-it-real-sync)
5. [Report what happened: Status](#5-report-what-happened-status)
6. [Say no early: Validate](#6-say-no-early-validate)
7. [Describe the form: the UI schema](#7-describe-the-form-the-ui-schema)
8. [Run it](#8-run-it)
9. [Test it](#9-test-it)
10. [Where to go next](#10-where-to-go-next)

---

## 1. What a provider is

OpenEverest gives users one API for every data service: the `Instance` custom resource. A
provider is the piece that knows what a particular technology needs.

```
User creates an Instance
        │
        ▼
Provider runtime  ──►  your provider
                         Validate()  is this spec acceptable?
                         Sync()      make the cluster match it
                         Status()    what does the cluster actually look like?
                         Cleanup()   the Instance is going away
```

Your provider has two halves, and keeping them straight is most of the job:

- **`definition/`** — *what* you offer. Which components exist, which versions, which
  topologies, which parameters, and how the UI should render the form. This is compiled into a
  `Provider` custom resource at build time. The API server and the UI read that resource, never
  your Go code, so anything a user must see *before* an Instance exists belongs here.
- **`internal/provider/`** — *how* you deliver it. The four methods above.

A provider is its own program, running as a Deployment beside the OpenEverest core rather than
inside it. For where it sits and who calls what, see
[How a provider runs](PROVIDER_DEVELOPMENT.md#how-a-provider-runs).

---

## 2. Scaffold the project

```bash
go install github.com/openeverest/provider-sdk@latest

provider-sdk init \
  --name my-database \
  --module github.com/my-org/provider-my-database
```

`--name` is the **provider** — the technology, and the name users write in
`spec.providerRef.name`. The **project** name is derived from it by adding the prefix
(`provider-my-database`) and names the repository, the chart and the image.

The first commit of `provider-example` is exactly this output, unmodified, so
`git log -p` there shows you what the generator wrote versus what a provider author writes.

Two things about the scaffold are worth knowing now:

- Every lifecycle method is a `// TODO`. It compiles and runs; it just does nothing yet.
- `provider-sdk` is a Go tool dependency pinned in `go.mod`, separate from the runtime. If
  `make generate` ever produces less than you expect, check that pin first.

---

## 3. Declare what you deploy

Three files, in the order the runtime reads them.

### Identity and components

`definition/provider.yaml`:

```yaml
name: example

components:
  engine:
    type: memcached
```

`name` is the `Provider` resource's name, and must match `common.ProviderName` in
`internal/common/spec.go` — that is the name the running binary uses to look up its own
`Provider`. Getting these out of step is the most common way to end up with a provider that
starts cleanly and then ignores every Instance.

Note the distinction the field names are drawing. A **component** is a role: `engine`,
`proxy`, `configServer`. A **component type** is the software that role runs. A sharded
MongoDB provider has `engine` and `configServer` components that are both type `mongod` —
same software, different jobs. Here there is one of each, which is the simplest case, not the
general one.

### Versions

`definition/versions.yaml`:

```yaml
componentTypes:
  memcached:
    defaultVersion: "1.6.38"
    versions:
      - version: "1.6.38"
        image: memcached:1.6.38-alpine
      - version: "1.6.31"
        image: memcached:1.6.31-alpine

defaultVersion: "1.6.38"
versions:
  - name: "1.6.38"
    components:
      engine: "1.6.38"
```

Two catalogs with different jobs. `componentTypes` says which versions exist and which image
implements each. The top-level `versions` are **bundles**: named sets of component versions
known to work together. Users pick a bundle with `spec.version` rather than pinning every
component — the difference between "PostgreSQL 17" and "PostgreSQL 17.2 with pgBouncer 1.23
and pgBackRest 2.54".

The runtime expands the bundle *before* it calls your code, so `Sync` always sees a concrete
version on each component and never has to know bundles exist.

### Parameters

`definition/components/types.go`:

```go
type MemcachedParameters struct {
	MaxConnections *int32 `json:"maxConnections,omitempty"`
	Threads        *int32 `json:"threads,omitempty"`
}
```

Referenced from `provider.yaml` as `parametersSchema: MemcachedParameters`. At generate time
the Go type becomes an OpenAPI schema on the `Provider` resource, which is what validates
`spec.components.engine.parameters` server-side. Write the type; the schema follows.

Notice what is *not* a parameter: memcached's memory budget. It is derived from the
component's resource limits in the next section, because two fields that must agree eventually
will not. The rule generalises — **before adding a parameter, check whether the Instance
already has a field for it.** `resources`, `replicas`, `storage` and `affinity` are part of
every Instance. Parameters are for what the engine understands and the platform does not.

Now generate:

```bash
make generate
git diff charts/*/generated/provider-spec.yaml
```

Everything you just declared is in that one generated file. Nothing is running yet.

---

## 4. Make it real: Sync

`Sync` runs on every reconciliation and must be idempotent: describe the objects the spec
implies, apply them, return. See
[`internal/provider/sync.go`](https://github.com/openeverest/provider-example/blob/main/internal/provider/sync.go).

```go
func sync(c *controller.Context) error {
	engine, err := resolveEngine(c)
	if err != nil {
		return err
	}
	if err := c.Apply(buildService(engine)); err != nil {
		return fmt.Errorf("apply service: %w", err)
	}
	return c.Apply(buildStatefulSet(engine))
}
```

`Context.Apply` creates or updates, and sets a controller reference back to the Instance. That
one detail pays for itself three times: Kubernetes garbage collects the objects on delete,
`WatchOwned` can route their events back to the Instance, and `Cleanup` stays empty.

Splitting `resolveEngine` (talks to the cluster) from the builders (pure functions of a
struct) is what makes section 9 possible without a cluster.

Three things `resolveEngine` works out:

**The image** — explicit `component.Image` wins, then the resolved version, then the catalog
default:

```go
image := controller.GetImageForVersion(providerSpec, common.ComponentEngine, component.Version)
if image == "" {
	image = controller.GetDefaultImageForComponent(providerSpec, common.ComponentEngine)
}
```

**The replica count.** The `defaults:` block in `topology.yaml` is documentation and a UI
hint — the generator strips it from the `Provider` resource, and nothing defaults component
fields at runtime. If you want a default, apply it here.

**The parameters**, decoded into the type you declared:

```go
var params components.MemcachedParameters
if c.TryDecodeComponentParameters(component, &params) { ... }
```

Two details worth copying. The StatefulSet's selector labels are deliberately separate from
the ones `Context.ObjectMeta` adds: a selector is immutable, so it must not depend on labels
the runtime might change. And the pod runs with `runAsNonRoot`, an explicit uid, a read-only
root filesystem and all capabilities dropped — providers create the workloads, so providers
own their security context.

The pod template also carries the labels `c.PodLabels(common.ComponentEngine)` returns. The
runtime counts the pods that have them into the Instance's `status.components`, and reports a
pod stuck in a crash loop or waiting for a node on the Instance, so `Status` never has to.

Finally, tell the manager about the types you use. The runtime's scheme knows the OpenEverest
APIs and `core/v1`; anything else is yours to register:

```go
SchemeFuncs:  []func(*runtime.Scheme) error{appsv1.AddToScheme},
WatchConfigs: []controller.WatchConfig{controller.WatchOwned(&appsv1.StatefulSet{})},
```

Skip the watch and the Instance is only reconciled when it changes — so the next section would
never notice pods becoming ready.

Then grant yourself the access in `rbac.go` and re-run `make generate`. The runtime needs to
read the pods to report on them:

```go
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
```

> **Grant yourself Secrets too**, even if you never touch one. The runtime writes the
> connection details from the next section into a Secret using the manager's *cached* client,
> so without `secrets` access the informer never syncs and every reconcile that reports
> connection details fails. The symptom is an Instance that never becomes `Ready`, and after
> a minute reports `ReconcileError`, while the workload is plainly healthy.

---

## 5. Report what happened: Status

`Sync` says what should exist. `Status` says what does, as facts: has the workload caught up
with the spec, can clients connect, how many nodes should there be. The runtime turns them
into the Instance's conditions and phase, the same way for every provider, and persists the
connection details into a Secret the API server serves to clients. See
[`internal/provider/status.go`](https://github.com/openeverest/provider-example/blob/main/internal/provider/status.go).

```go
func statusFromStatefulSet(sts *appsv1.StatefulSet, desired int32, details controller.ConnectionDetails) controller.Status {
	ready := sts.Status.ReadyReplicas

	var status controller.Status
	switch {
	case sts.Status.ObservedGeneration < sts.Generation:
		status = controller.Progressing("Rolling out the updated configuration")
	case sts.Status.CurrentRevision != sts.Status.UpdateRevision:
		status = controller.Progressing(fmt.Sprintf("%d of %d nodes updated", sts.Status.UpdatedReplicas, desired))
	case sts.Status.Replicas != desired || ready < desired:
		status = controller.Progressing(fmt.Sprintf("%d of %d nodes ready", ready, desired))
	default:
		status = controller.Ready()
	}

	if ready == 0 {
		status = status.WithUnavailable("NoNodeReady", "No memcached node is ready")
	} else {
		status = status.WithAvailable(controller.Available)
		status.ConnectionDetails = details
	}

	return status.WithDesiredReplicas(common.ComponentEngine, desired)
}
```

The UI, the CLI and anything waiting on an Instance read what the runtime derives from these
facts, so each has to mean something specific:

| Fact | Means | Here |
|---|---|---|
| Converged: `Ready()`, or `Progressing(message)` until then | The workload runs what the spec asks for. Until it does, the message says what it waits for. | The StatefulSet has acted on its latest generation, every pod runs the latest revision, and every node is ready. |
| Available | Clients can connect right now. | At least one node is ready. |
| Connection details | How clients connect. | Reported as soon as a node is ready, not only once converged. |
| Desired replicas | How many pods the component has once converged. | The node count the Instance asks for, with the default `Sync` applies. |

From them the runtime derives the `Ready` and `Available` conditions and the phase: `Pending`
until the cache has first been available, then `Provisioned` for good. `Ready`'s reason is the
status word clients show: `Creating` while the Instance first comes up, `ApplyingChanges` while
a later change rolls out, and `Ready` once it has converged.

The generation check comes first on purpose. Immediately after a spec change the StatefulSet
still reports the *old* replica count as fully ready, so a check that skipped it would report
the Instance converged for a rollout that has not started. The revision check catches the
moments in a rolling update when every pod happens to be ready but some still run the old
template. And `desired` comes from the Instance, through the `resolveReplicas` helper `Sync`
uses, not from the StatefulSet, which may still describe the previous spec.

A missing workload is not an error — `Sync` ran, the cache has not caught up:

```go
if apierrors.IsNotFound(err) {
	return controller.Progressing("Waiting for the StatefulSet to be created"), nil
}
```

Any other failed read is returned as an error. The runtime retries it, and reports it only if
it persists.

memcached has no operator to report errors or long operations, so this provider reports
neither. One backed by an operator adds them: `WithProblem` for an error the operator reports,
`WithOperation` for an upgrade, a restore or a volume resize, as
[Status](PROVIDER_DEVELOPMENT.md#status) in the guide shows. Neither reports a crash loop or a
pod that cannot be scheduled: the runtime sees those on the pods labelled in section 4.

Fill in the well-known connection fields so the API server can serve them without knowing
anything about your engine. `AdditionalProperties` carries whatever they do not cover; every
entry becomes a key in the connection Secret.

---

## 6. Say no early: Validate

Before every `Sync`, the runtime checks the Instance against what your `Provider` declares —
topologies, components, versions, parameter schemas and `supportedFields` — and then calls
`Validate` with the Instance and the `Provider`. `Validate` gets no client: it judges the
Instance alone and returns every error with its field path.

```go
if replicas > maxPoolNodes {
	errs = append(errs, field.Invalid(
		field.NewPath("spec", "components").Key("engine").Child("replicas"), replicas,
		fmt.Sprintf("a pool supports at most %d nodes", maxPoolNodes),
	))
}
```

A rejected Instance keeps its phase and reports `Ready=False` with reason `ValidationFailed`
and your errors, and `Stalled=True`, so `kubectl wait`, Argo CD and Flux all see the
rejection, and nothing is synced until the Instance or the `Provider` changes. Say what is wrong *and* what to do
instead — that message is the entire user experience of a rejection, whether it lands in a
form or in `kubectl describe`.

A single-field bound like this one can also be declared, as `maximum` on the field in the
component's `supportedFields`, and then the form enforces it as the user types as well.
Declare what you can; write in `Validate` what spans fields.

---

## 7. Describe the form: the UI schema

The `ui:` half of each `topology.yaml` tells the OpenEverest frontend how to render the
create/edit form. It lives next to the topology because the same field can reasonably look
different in different deployment shapes.

```yaml
version:
  uiType: select
  path: spec.version
  fieldParams:
    label: Version
    optionsPath: spec.versions          # read from the Provider resource itself
    optionsPathConfig: {labelPath: name, valuePath: name}
    modes:
      edit:
        disabled: true                  # versions are chosen once
  validation:
    required: true
```

`optionsPath` is the one to notice: the dropdown is populated from the `Provider` resource, so
adding a bundle to `versions.yaml` makes it selectable with no frontend change.

Rules that span fields are CEL, and can compare against the persisted value in edit mode:

```yaml
modes:
  edit:
    celExpressions:
      - celExpr: spec.components.engine.replicas >= original.spec.components.engine.replicas
        message: Nodes cannot be removed from a running pool.
```

Anything the form enforces should also be enforced on the server, declared in a schema or
written in `Validate`. The form is a courtesy; the server is the contract.

The full vocabulary — field types, groups, modes, data sources, validation rules — is in
[the guide](PROVIDER_DEVELOPMENT.md#configure-the-ui-schema).

### A note on topologies

A topology is a deployment **architecture**: a distinct set of components, or a materially
different way of wiring them together. `provider-example` declares one, because memcached has
one architecture. Running three nodes instead of one is
`spec.components.engine.replicas` — a number on a component, not a different way of assembling
the system.

The contrast is `provider-percona-server-mongodb`, whose `sharded` topology adds `proxy` and
`configServer` components that `replicaSet` does not have, plus a `numShards` parameter that
is meaningless in the other shape. Reach for a second topology when the component set changes,
not when a count does. Most providers never need one.

---

## 8. Run it

```bash
make dev-up
```

That creates a k3d cluster, installs the released OpenEverest core, and deploys your provider
with live reload through [Tilt](https://tilt.dev). Then:

```bash
kubectl apply -f examples/instance-simple.yaml
kubectl get instance cache -w
```

You should watch its `STATUS` go from `Creating` to `Ready`, and `AVAILABLE` turn `True`
as soon as the first node is ready. From then on the connection Secret is populated:

```bash
kubectl wait --for=condition=Ready instance/cache --timeout=5m
kubectl get secret cache-conn -o jsonpath='{.data.uri}' | base64 -d
```

If it never reaches `Ready`, `kubectl describe instance cache` shows the conditions, and the
`Ready` condition's message says what it waits for. The two usual causes are the missing
`secrets` RBAC rule from section 4 and a missing watch, which leaves `Status` running only
when the Instance itself changes.

---

## 9. Test it

**Unit tests** cover the half that is a pure function of the spec — which is why section 4
split `resolveEngine` from the builders. A fake client serves the `Provider` resource; there
is nothing else to stand in for.

```go
engine, err := resolveEngine(newTestContext(t, testInstance(common.TopologyPool, component)))
require.NoError(t, err)
assert.Equal(t, "memcached:1.6.38-alpine", engine.image)
```

Assert values the code *chose*: the resolved image, the derived flags, the facts a given
StatefulSet status maps to — converged or not, available or not, the desired replicas. A test that only asserts "no error" tells you nothing.

**Integration tests** are [chainsaw](https://kyverno.github.io/chainsaw/) suites under
`test/integration/`, run in CI against a real k3d cluster:

```bash
make test-unit
make test-integration-core
```

---

## 10. Where to go next

You now have the whole contract. What is left is depth, and each of these is a section in the
[guide](PROVIDER_DEVELOPMENT.md):

- [Back a real operator](PROVIDER_DEVELOPMENT.md#implement-the-provider-interface) — swap the
  StatefulSet for an operator's custom resource. Only `Sync` and `Status` change, which is the
  point of everything above them.
- [Add persistent storage](PROVIDER_DEVELOPMENT.md#implement-the-provider-interface) from
  `spec.components[].storage`, remembering that storage grows but never shrinks.
- [Add backups and restores](PROVIDER_DEVELOPMENT.md#add-backup-and-restore-support).
- [Add presets](PROVIDER_DEVELOPMENT.md#define-presets) so users can start from a known-good
  configuration.
- [Release it](PROVIDER_DEVELOPMENT.md#release-and-publish).

For a provider that does all of this in production, read
[provider-percona-server-mongodb](https://github.com/openeverest/provider-percona-server-mongodb).
