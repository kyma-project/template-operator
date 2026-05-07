# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this repo is

template-operator is the **reference implementation for Kyma module operators**. It is not a production component — it is the canonical starting point developers copy when building a new Kyma module. Any pattern here is intentional and should be treated as the recommended way to implement a Kyma module operator.

## Module & language

- Module: `github.com/kyma-project/template-operator` (Go 1.26.1)
- API is a **separate Go module**: `github.com/kyma-project/template-operator/api`
- Key dependencies: `controller-runtime v0.24.0`, `k8s.io/* v0.36.0`
- Tool versions pinned in `versions.yaml`

## Make targets

Run from the repo root:

| Target | What it does |
|---|---|
| `make generate` | Regenerate `zz_generated.deepcopy.go` via controller-gen |
| `make manifests` | Regenerate CRD YAML (`config/crd/bases/`) and RBAC via controller-gen |
| `make test` | Run all tests with envtest (also runs generate/manifests/fmt/vet) |
| `make build` | Run generate, fmt, vet, lint, then compile `bin/manager` |
| `make lint` | golangci-lint on root and `api/` |
| `make fmt` / `make vet` | `go fmt` / `go vet` |
| `make build-manifests` | Build static `template-operator.yaml` (Deployment) for release |
| `make build-module` | Build Kyma module artifact via `kyma alpha create module` |

**After any type change in `api/`**: run both `make generate` and `make manifests`.

### Running a single test

```sh
KUBEBUILDER_ASSETS=$(./bin/setup-envtest use 1.32.0 -p path) \
  go test ./controllers/... -v -ginkgo.focus "some spec description"
```

Run `make envtest` once after checkout to populate `bin/setup-envtest`.

## Architecture

### Single controller

`controllers/sample_controller_rendered_resources.go` — `SampleReconciler` is the only controller. It:
1. Reads YAML manifests from the path specified in `Sample.spec.resourceFilePath`
2. Parses them into unstructured objects
3. Applies them to the cluster using **Server-Side Apply (SSA)** — never `Create`/`Update` directly
4. Transitions the `Sample` CR through the state machine

### State machine

```
(new) ──► Processing ──► Ready
                    └──► Error ──► (retry → Ready)
                    └──► Deleting ──► (finalizer removed)
```

Final states are configurable via flags: `--final-state` (default `Ready`) and `--final-deletion-state` (default `Deleting`). Tests set `FinalDeletionState=Warning` to verify warning path.

### CRD — `Sample` (`api/v1alpha1`)

```
operator.kyma-project.io/v1alpha1/Sample
```

- `spec.resourceFilePath` — local directory containing exactly one `.yaml`/`.yml` file to apply
- `status.state` — enum: `Ready | Processing | Error | Deleting | Warning`
- `status.conditions` — standard `[]metav1.Condition`; single condition: `Installation`

Finalizer: `sample.kyma-project.io/finalizer`
SSA field owner: `sample.kyma-project.io/owner`

### Deployment modes

Two kustomize overlays — `config/overlays/deployment/` and `config/overlays/statefulset/`. Use `make deploy` or `make deploy-statefulset`. Both inject `--final-state` and `--final-deletion-state` args.

## Code conventions

- **Status updates use SSA** (`ssaStatus`) — never `r.Status().Update()` directly
- **Resource application uses SSA** (`ssa`) — never `r.Create()` / `r.Update()` for managed resources
- **Conditions** set via `status.WithInstallConditionStatus(metav1.ConditionTrue/False, generation)`
- **RBAC markers** live in the controller file, not in `api/`

## Testing

Tests live in `controllers/` alongside the controller (not in a separate `tests/` directory). The suite bootstrap in `suite_test.go` wires the full `SampleReconciler` against an envtest environment. Test fixtures (busybox manifests) are in `controllers/test/busybox/manifest/`.
