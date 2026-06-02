---
paths:
  - "**/*.go"
---

# Go code conventions — template-operator

`make lint` is the authoritative check. The full config is in `.golangci.yaml`.

## nolint policy

Every `//nolint` directive **must** include an explanation:

```go
//nolint:funlen // controller setup — acceptable exception
```

Bare suppressions fail CI. Check `.golangci.yaml` before adding any.

## Import ordering

gci enforces: standard → third-party → project (`github.com/kyma-project/template-operator`).

Ginkgo and Gomega dot-imports are whitelisted in tests — do not add dot-imports for other packages.

## Status and conditions

State is communicated via `status.state` (`Ready | Processing | Deleting | Error`) and `status.conditions`. Do not use free-form status strings as the primary signal.

## After type changes in `api/`

Run `make generate && make manifests` and commit the updated `zz_generated.deepcopy.go` and `crd/*.yaml`.
