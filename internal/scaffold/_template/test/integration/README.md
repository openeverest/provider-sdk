# Integration tests

Integration tests exercise the provider against a real Kubernetes cluster
using [chainsaw](https://kyverno.github.io/chainsaw/). They apply OpenEverest
`Instance` CRs, assert on the operator-native resources the provider produces,
and verify status propagation back to the `Instance`.

This directory is a **skeleton**: the `core/` suite ships with a single step
that verifies the provider deployment is running, plus commented-out sample
steps showing how to test the full Instance lifecycle. Flesh it out as you
implement your provider. For a complete, real-world example see the
[provider-percona-server-mongodb test suites](https://github.com/openeverest/provider-percona-server-mongodb/tree/main/test/integration).

## Layout

```
test/
  vars.sh                      # Pinned operator/engine versions (sourced by make)
  integration/
    .chainsaw.yaml             # Shared chainsaw configuration (timeouts, reports)
    core/                      # Core lifecycle suite (create/update/delete)
      chainsaw-test.yaml       # Test definition: ordered steps
      00-assert.yaml           # Step files, prefixed by execution order
      ...
```

Each suite is a directory containing one `chainsaw-test.yaml` plus the
manifest/assert files its steps reference. Add more suites as sibling
directories (e.g. `backup/`, `monitoring/`) and wire each one to its own
`test-integration-<suite>` Make target and CI job.

## Conventions

- **Numbered step files** — the `NN-` prefix mirrors the step order in
  `chainsaw-test.yaml`: `10-create-instance.yaml` applies a manifest,
  `10-assert.yaml` asserts the expected outcome of that step.
- **Simulate the operator** — deploy the provider with your DB operator
  scaled to 0 (see `deploy-provider-ci` in the Makefile). Tests then patch the
  operator-native CR status (`kubectl patch --subresource status`) to simulate
  readiness. This keeps suites fast and deterministic: you are testing the
  *provider's* translation and status logic, not the operator itself.
- **Assert both directions** — after applying an `Instance`, assert the
  operator-native CR spec the provider must produce; after patching the
  operator CR status, assert the `Instance` reaches `phase: Running`.
- **Collect diagnostics on failure** — use the `catch` block in
  `chainsaw-test.yaml` to dump relevant resources and provider logs.

## Running locally

Prerequisites: a running cluster with the OpenEverest CRDs, the OpenEverest
controller, your operator's CRDs, and the provider deployed (see the
`install-crds` and `deploy-provider-ci` Make targets), plus the
[chainsaw CLI](https://kyverno.github.io/chainsaw/latest/quick-start/install/):

```bash
go install github.com/kyverno/chainsaw@latest
```

Then:

```bash
make test-integration        # all suites
make test-integration-core   # just the core suite
```

## Running in CI

`.github/workflows/ci.yaml` runs each suite as a separate job through the
reusable `.github/workflows/integration-test.yaml` workflow, which provisions
a k3d cluster, builds and deploys the provider and the OpenEverest controller,
and invokes the corresponding Make target. To add a suite to CI, add a job to
`ci.yaml` that passes the suite's `make_target`.
