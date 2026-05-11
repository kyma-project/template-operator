---
paths:
  - "**/*.go"
---

# Go code conventions — template-operator

`make lint` is the authoritative check. The full linter config is in `.golangci.yaml`.

## nolint policy

Every `//nolint` directive **must** include an explanation:
```go
//nolint:funlen // reconcile loop — acceptable exception
```
Bare `//nolint:funlen` fails CI (`nolintlint` is enabled). Check `.golangci.yaml` before adding any suppression.

## SSA — never use Create/Update for managed resources

All resource creation and updates must go through Server-Side Apply:
```go
// correct
r.Client.Patch(ctx, obj, client.Apply, client.ForceOwnership, client.FieldOwner("sample.kyma-project.io/owner"))

// wrong — breaks ownership semantics
r.Client.Create(ctx, obj)
r.Client.Update(ctx, obj)
```
Status updates use `ssaStatus`, not `r.Status().Update()`. This is the pattern module teams copy — keep it correct.
