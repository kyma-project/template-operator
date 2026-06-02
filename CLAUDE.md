# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this repo is

Template Operator is a **reference implementation and tutorial** for building Kyma module operators. It demonstrates the patterns, structure, and conventions that all Kyma modules must follow to integrate with [Lifecycle Manager](https://github.com/kyma-project/lifecycle-manager).

It is a kubebuilder-based Kubernetes operator written in Go, deployed to SKR clusters and managed through Lifecycle Manager via the `Manifest` CR. When implementing a new Kyma module, this repository is the starting point.

## Two Go modules

| Directory | Module | Role |
|---|---|---|
| `./` | `github.com/kyma-project/template-operator` | Operator binary — controller, main entrypoint |
| `api/` | `github.com/kyma-project/template-operator/api` | CRD types — separate module consumed via local `replace` |

Run `go` and `make` commands from the repo root. The root `Makefile` handles both modules. Tool versions are centralized in `versions.yaml`.

## Custom Resources

- **`Sample`** (`api/v1alpha1`) — the primary demo CR. Its `spec.resourceFilePath` points to a directory of YAML resources; the controller installs those resources onto the cluster and tracks the result via `status.state` and an `Installation` condition.
- **`ThirdParty`** (`api/v1alpha1`) — demonstrates watching and reacting to an externally-owned resource.

Both types follow the shared Kyma status pattern: `status.state` uses `Ready | Processing | Deleting | Error` and communicates through `status.conditions`.

## Make targets

| Target | What it does |
|---|---|
| `make build` | `generate` + `fmt` + `vet` + `lint` + compile binary to `bin/manager` |
| `make test` | `manifests` + `generate` + `fmt` + `vet` + envtest (Ginkgo) |
| `make lint` | golangci-lint on root and `api/` |
| `make manifests` | Regenerate CRD YAML and RBAC from controller-gen markers |
| `make generate` | Regenerate `zz_generated.deepcopy.go` |
| `make run` | Run controller locally against current kubeconfig |
| `make deploy` / `make undeploy` | Deploy/remove controller from cluster |
| `make build-module` | Build the module as an OCI artifact via `modulectl` |
| `make bump-go-version GO_VERSION=x.y.z` | Bump Go version |

**After any type change in `api/`:** always run `make generate && make manifests` and commit the updated `zz_generated.deepcopy.go` and `crd/*.yaml`.

### Running a single test

```sh
KUBEBUILDER_ASSETS="$(./bin/setup-envtest use $(yq e '.envtest_k8s' versions.yaml) -p path)" \
  go test -run TestFoo ./controllers/... -v
```

## Controller behaviour

The `Sample` controller:
1. Reads `spec.resourceFilePath` to find YAML resources to deploy
2. Renders and applies them to the cluster via SSA (server-side apply)
3. Tracks readiness and reflects it in `status.state` and the `Installation` condition
4. On deletion, removes all managed resources before clearing the finalizer

Conditions use `metav1.Condition` with `conditionType: Installation`. Never write free-form strings as the primary state signal — use `status.state` + conditions.

## Module integration

Template Operator ships as a Kyma module, meaning:
- It is packaged as an OCI artifact via `modulectl create`
- Lifecycle Manager installs it on SKRs through a `Manifest` CR
- Its release channels and versions are declared via `ModuleTemplate` and `ModuleReleaseMeta` CRs in KCP
- `module-config.yaml` and `module-release-meta.yaml` define the module metadata

## Code conventions

Go conventions load automatically when editing `.go` files — see [`.claude/rules/go-conventions.md`](.claude/rules/go-conventions.md).

Key rules from `.golangci.yaml`:
- **All linters enabled by default** — check `.golangci.yaml` before adding `//nolint`
- **`//nolint` requires explanation**: e.g., `//nolint:funlen // controller setup`
- **Import ordering** (gci): standard → third-party → project (`github.com/kyma-project/template-operator`)
- **Line length**: 120 chars | **Function length**: 80 lines / 50 statements | **Cyclomatic complexity**: 20
- Ginkgo/Gomega dot-imports are whitelisted

## Commits and Pull Requests

- PRs are usually created from a **fork branch** against `main`.
- PRs are merged with **squash merge** — the PR title and description form the commit message.
- Follow [conventional commits](https://www.conventionalcommits.org/), enforced by `.github/workflows/lint-conventional-prs.yml`.
- PR title format: `<type>: <title>` where the title is one sentence explaining the reason for the changeset.
- Ask what type to use when creating a PR: `deps`, `chore`, `docs`, `feat`, `fix`, `refactor`, `test`.
- PR description should contain a short summary of the changes and, if applicable, a reference to the issue using the `closes` or `resolves` keyword.
- Never mention Claude or any AI agent in commits or PRs (no author attribution, no `Co-Authored-By`, no references in commit messages).

## Documentation

When reviewing or editing documentation in `docs/`, the SAP/Kyma technical writing styleguide loads automatically — see [`.claude/rules/documentation-style.md`](.claude/rules/documentation-style.md).

Detailed docs in `docs/`:
- `contributor/` — development setup, local testing guide
- `user/` — end-user documentation
