# Build the manager binary
FROM golang:1.24.5-alpine as builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /template-operator
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# Copy the go source
COPY main.go main.go
COPY api api/
COPY controllers controllers/
COPY module-data module-data/
RUN chmod 755 module-data/

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

ARG TAG_default_tag=from_dockerfile

# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -ldflags="-X 'main.buildVersion=${TAG_default_tag}'" -a -o manager main.go
# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /

COPY --chown=65532:65532 --from=builder /template-operator/manager .
COPY --chown=65532:65532 --from=builder /template-operator/module-data module-data/
USER 65532:65532

ENTRYPOINT ["/manager"]
