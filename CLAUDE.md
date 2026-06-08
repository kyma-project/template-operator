# CLAUDE.md

Template Operator is a **reference implementation and tutorial** for building Kyma module operators. It demonstrates the patterns, structure, and conventions that all Kyma modules must follow to integrate with [Lifecycle Manager](https://github.com/kyma-project/lifecycle-manager).

It is a kubebuilder-based Kubernetes operator written in Go, deployed to SAP BTP, Kyma Runtime (SKR) clusters and managed through Lifecycle Manager via the `Manifest` CR. When implementing a new Kyma module, this repository is the starting point.

To build the operator binary, run `make build`.

## template-operator consists of two Go modules

| Directory | Module | Role |
|---|---|---|
| `./` | `github.com/kyma-project/template-operator` | Operator binary — controller, main entrypoint |
| `api/` | `github.com/kyma-project/template-operator/api` | CRD types — separate module consumed via local `replace` |

Run `go` and `make` commands from the repo root. The root `Makefile` handles both modules. Tool versions are centralized in `versions.yaml`.

**After any type change in `api/`:** always run `make generate && make manifests` and commit the updated `zz_generated.deepcopy.go` and `crd/*.yaml`.

## template-operator manages the following Custom Resources

- **`Sample`** (`api/v1alpha1`) — the primary demo CR that the operator manages.
- **`ThirdParty`** (`api/v1alpha1`) — demonstrates watching and reacting to an externally-owned resource.

Both types follow the Kyma status pattern: `status.state` uses `Ready | Processing | Deleting | Error` communicated through `status.conditions`. Never write free-form strings as the primary state signal — use `status.state` + conditions.

## template-operator uses the following code conventions

Go nolint and import ordering rules load automatically when editing `.go` files — see [`.claude/rules/go-conventions.md`](.claude/rules/go-conventions.md).

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
