SHELL := /usr/bin/env bash
export GOFLAGS := -buildvcs=false
.DEFAULT_GOAL := build

CONTROLLER_GEN_VERSION := v0.20.0
KUSTOMIZE_VERSION := v5.8.1
SETUP_ENVTEST_VERSION := v0.24.2-0.20260713111223-0f529e22d5c0
GOLANGCI_LINT_VERSION := v2.5.0
GOVULNCHECK_VERSION := v1.1.4
GOSEC_VERSION := v2.22.10
KO_VERSION := v0.18.0
SYFT_VERSION := v1.33.0
TRIVY_VERSION := v0.67.2

BIN := $(CURDIR)/bin
CONTROLLER_GEN := $(BIN)/controller-gen
KUSTOMIZE := $(BIN)/kustomize

.PHONY: generate manifests fmt vet lint test test-race test-envtest test-e2e build image image-load install uninstall deploy undeploy bundle sbom scan verify tools

$(CONTROLLER_GEN):
	GOBIN=$(BIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION)

$(KUSTOMIZE):
	GOBIN=$(BIN) go install sigs.k8s.io/kustomize/kustomize/v5@$(KUSTOMIZE_VERSION)

generate: $(CONTROLLER_GEN)
	cd api && $(CONTROLLER_GEN) object:headerFile=/dev/null paths=./...

manifests: $(CONTROLLER_GEN)
	cd api && $(CONTROLLER_GEN) crd:crdVersions=v1 paths=./... output:crd:artifacts:config=../config/crd/bases

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './bin/*')

vet:
	go vet ./...

lint:
	GOBIN=$(BIN) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	GOFLAGS=-buildvcs=false $(BIN)/golangci-lint run

test:
	go test ./...
	cd api && go test ./...

test-race:
	go test -race ./...

test-envtest:
	GOBIN=$(BIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@$(SETUP_ENVTEST_VERSION)
	KUBEBUILDER_ASSETS="$$($(BIN)/setup-envtest use --bin-dir $(CURDIR)/.envtest -p path 1.36.x!)" go test -tags=envtest ./internal/controller/...

test-e2e:
	./tests/e2e/run.sh

build:
	CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags "-s -w -X main.version=$${VERSION:-devel}" -o bin/mosaic-controller ./cmd/controller

image:
	GOBIN=$(BIN) go install github.com/google/ko@$(KO_VERSION)
	KO_DOCKER_REPO=$${KO_DOCKER_REPO:-ko.local} $(BIN)/ko build --bare --platform=linux/amd64,linux/arm64 --image-label org.opencontainers.image.title=mosaic-controller --image-label org.opencontainers.image.version=$${VERSION:-devel} --image-label org.opencontainers.image.revision=$${GIT_COMMIT:-unknown} ./cmd/controller

image-load:
	GOBIN=$(BIN) go install github.com/google/ko@$(KO_VERSION)
	KO_DOCKER_REPO=kind.local $(BIN)/ko build --bare --local --platform=linux/amd64 --image-label org.opencontainers.image.title=mosaic-controller --image-label org.opencontainers.image.version=$${VERSION:-devel} --image-label org.opencontainers.image.revision=$${GIT_COMMIT:-unknown} ./cmd/controller

install: manifests $(KUSTOMIZE)
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

uninstall: $(KUSTOMIZE)
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found -f -

deploy: manifests $(KUSTOMIZE)
	$(KUSTOMIZE) build config/default | kubectl apply -f -

undeploy: $(KUSTOMIZE)
	$(KUSTOMIZE) build config/default | kubectl delete --ignore-not-found -f -

bundle: manifests $(KUSTOMIZE)
	mkdir -p dist
	$(KUSTOMIZE) build config/default > dist/install.yaml

sbom:
	GOBIN=$(BIN) go install github.com/anchore/syft/cmd/syft@$(SYFT_VERSION)
	$(BIN)/syft $${IMAGE:?set IMAGE to an immutable image digest} -o spdx-json=sbom.spdx.json

scan:
	@echo "Install Trivy $(TRIVY_VERSION), then run: trivy image --exit-code 1 \$$IMAGE"

verify: generate manifests
	test -z "$$(gofmt -l $$(find . -name '*.go' -not -path './bin/*'))"
	go vet ./...
	go test ./...
	go test -race ./...
	GOBIN=$(BIN) go install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)
	GOFLAGS=-buildvcs=false $(BIN)/govulncheck ./...
	GOBIN=$(BIN) go install github.com/securego/gosec/v2/cmd/gosec@$(GOSEC_VERSION)
	GOFLAGS=-buildvcs=false $(BIN)/gosec -exclude-dir=.cache -exclude-dir=bin ./...
