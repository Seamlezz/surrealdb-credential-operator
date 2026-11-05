# Script names are shared with the other repos of this stack. They avoid `test` and `fmt`,
# which lose to the shell builtin and coreutils, and `install`, which loses to coreutils too.
# `devenv shell` gives Go, the Kubernetes tools and the scripts below. Image and e2e scripts
# use the host's Docker.
# `devenv test` runs the full local check.
{ pkgs, ... }:

let
  image = "ghcr.io/seamlezz/surrealdb-credential-operator";
  chartDir = "charts/surrealdb-credential-operator";
  chartRepository = "oci://ghcr.io/seamlezz/charts";
  kindCluster = "surrealdb-credential-operator-test-e2e";
  # The generated files record the generator's version, so it stays pinned here.
  controllerGen = "go run sigs.k8s.io/controller-tools/cmd/controller-gen@v0.19.0";
  # Not ./...: controller-gen would also walk the Go module cache in .devenv.
  sources = "./api/...;./cmd/...;./internal/...";
  # kind writes the current context, so e2e gets its own kubeconfig and leaves
  # the developer's real clusters alone.
  e2eKubeconfig = ''export KUBECONFIG="$DEVENV_STATE/kind-e2e.kubeconfig"'';
in
{
  languages.go.enable = true;

  packages = [
    pkgs.git
    pkgs.golangci-lint
    pkgs.kind
    pkgs.kubectl
    pkgs.kubernetes-helm
    pkgs.kustomize
    pkgs.setup-envtest
  ];

  scripts = {
    format = {
      description = "Format all Go code";
      exec = ''cd "$DEVENV_ROOT" && go fmt ./...'';
    };
    format-check = {
      description = "Fail when Go code is not formatted";
      exec = ''
        set -euo pipefail
        cd "$DEVENV_ROOT"
        unformatted="$(gofmt -l api cmd internal test)"
        if [ -n "$unformatted" ]; then
          echo "$unformatted" >&2
          exit 1
        fi
      '';
    };
    vet = {
      description = "Run go vet";
      exec = ''cd "$DEVENV_ROOT" && go vet ./...'';
    };
    lint = {
      description = "Run golangci-lint";
      exec = ''cd "$DEVENV_ROOT" && golangci-lint run'';
    };
    lint-fix = {
      description = "Run golangci-lint and apply fixes";
      exec = ''cd "$DEVENV_ROOT" && golangci-lint run --fix'';
    };
    lint-config = {
      description = "Verify the golangci-lint configuration";
      exec = ''cd "$DEVENV_ROOT" && golangci-lint config verify'';
    };
    manifests = {
      description = "Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects";
      exec = ''cd "$DEVENV_ROOT" && ${controllerGen} rbac:roleName=manager-role crd webhook paths="${sources}" output:crd:artifacts:config=config/crd/bases'';
    };
    generate = {
      description = "Generate DeepCopy, DeepCopyInto and DeepCopyObject methods";
      exec = ''cd "$DEVENV_ROOT" && ${controllerGen} object:headerFile="hack/boilerplate.go.txt" paths="${sources}"'';
    };
    setup-envtest-assets = {
      description = "Download the envtest binaries for the Kubernetes version in go.mod and print their path";
      exec = ''
        set -euo pipefail
        cd "$DEVENV_ROOT"
        k8s="$(go list -m -f '{{if .Replace}}{{.Replace.Version}}{{else}}{{.Version}}{{end}}' k8s.io/api | sed -E 's/^v?[0-9]+\.([0-9]+).*/1.\1/')"
        setup-envtest use "$k8s" --bin-dir "$DEVENV_ROOT/bin" -p path
      '';
    };
    tests = {
      description = "Generate code, vet and run the unit and envtest tests";
      exec = ''
        set -euo pipefail
        cd "$DEVENV_ROOT"
        manifests
        generate
        format
        vet
        assets="$(setup-envtest-assets)"
        KUBEBUILDER_ASSETS="$assets" go test $(go list ./... | grep -v /e2e) -coverprofile cover.out
      '';
    };
    chart = {
      description = "Lint and render the Helm chart";
      exec = ''
        set -euo pipefail
        cd "$DEVENV_ROOT"
        helm lint ${chartDir}
        helm template surrealdb-credential-operator ${chartDir} >/dev/null
      '';
    };
    build = {
      description = "Generate code and build the manager binary";
      exec = ''
        set -euo pipefail
        cd "$DEVENV_ROOT"
        manifests
        generate
        format
        vet
        go build -o bin/manager cmd/main.go
      '';
    };
    run = {
      description = "Generate code and run the controller from this machine";
      exec = ''
        set -euo pipefail
        cd "$DEVENV_ROOT"
        manifests
        generate
        format
        vet
        go run ./cmd/main.go
      '';
    };
    check = {
      description = "Format check, vet, test and render everything";
      exec = ''
        set -euo pipefail
        format-check
        vet
        tests
        chart
      '';
    };
    setup-test-e2e = {
      description = "Create the kind cluster for e2e tests if it does not exist";
      exec = ''
        set -euo pipefail
        ${e2eKubeconfig}
        if kind get clusters | grep -qx ${kindCluster}; then
          echo "Kind cluster ${kindCluster} already exists."
        else
          kind create cluster --name ${kindCluster}
        fi
      '';
    };
    cleanup-test-e2e = {
      description = "Delete the kind cluster used for e2e tests";
      exec = ''
        ${e2eKubeconfig}
        kind delete cluster --name ${kindCluster}
      '';
    };
    test-e2e = {
      description = "Run the e2e tests against a throwaway kind cluster; the manager runs on this machine";
      exec = ''
        set -euo pipefail
        cd "$DEVENV_ROOT"
        ${e2eKubeconfig}
        setup-test-e2e
        trap cleanup-test-e2e EXIT
        E2E_LOCAL_MANAGER=true go test -tags=e2e ./test/e2e -v -count=1 -timeout=12m
      '';
    };
    install-crds = {
      description = "Install the CRDs into the current cluster";
      exec = ''
        set -euo pipefail
        cd "$DEVENV_ROOT"
        manifests
        kustomize build config/crd | kubectl apply -f -
      '';
    };
    uninstall-crds = {
      description = "Remove the CRDs from the current cluster";
      exec = ''
        set -euo pipefail
        cd "$DEVENV_ROOT"
        manifests
        kustomize build config/crd | kubectl delete --ignore-not-found -f -
      '';
    };
    deploy = {
      description = "Deploy the controller to the current cluster; IMG picks the image";
      exec = ''
        set -euo pipefail
        cd "$DEVENV_ROOT"
        manifests
        (cd config/manager && kustomize edit set image controller="''${IMG:-controller:latest}")
        kustomize build config/default | kubectl apply -f -
      '';
    };
    undeploy = {
      description = "Remove the controller from the current cluster";
      exec = ''
        set -euo pipefail
        cd "$DEVENV_ROOT"
        kustomize build config/default | kubectl delete --ignore-not-found -f -
      '';
    };
    build-installer = {
      description = "Write the CRDs and deployment to dist/install.yaml; IMG picks the image";
      exec = ''
        set -euo pipefail
        cd "$DEVENV_ROOT"
        manifests
        generate
        mkdir -p dist
        (cd config/manager && kustomize edit set image controller="''${IMG:-controller:latest}")
        kustomize build config/default > dist/install.yaml
      '';
    };
    docker-build = {
      description = "Build the image; IMG picks the tag";
      exec = ''cd "$DEVENV_ROOT" && docker build --tag "''${IMG:-controller:latest}" .'';
    };
    docker-push = {
      description = "Push the image; IMG picks the tag";
      exec = ''docker push "''${IMG:-controller:latest}"'';
    };
    ghcr-login = {
      description = "Log Docker and Helm in to ghcr.io with GHCR_USERNAME and GHCR_TOKEN";
      exec = ''
        set -euo pipefail
        printf '%s' "$GHCR_TOKEN" | docker login ghcr.io --username "$GHCR_USERNAME" --password-stdin
        printf '%s' "$GHCR_TOKEN" | helm registry login ghcr.io --username "$GHCR_USERNAME" --password-stdin
      '';
    };
    publish-image = {
      description = "Build linux/amd64 and linux/arm64 and push them under each given tag";
      exec = ''
        set -euo pipefail
        cd "$DEVENV_ROOT"
        tags=()
        for tag in "$@"; do tags+=(--tag "${image}:$tag"); done
        builder="surrealdb-credential-operator-builder-$$"
        docker buildx create --name "$builder" >/dev/null
        trap 'docker buildx rm "$builder"' EXIT
        # The Dockerfile cross compiles, so neither platform runs under emulation.
        docker buildx build --builder "$builder" --push --provenance=false \
          --platform linux/amd64,linux/arm64 "''${tags[@]}" .
      '';
    };
    publish-chart = {
      description = "Package the chart as <version> and push it";
      exec = ''
        set -euo pipefail
        cd "$DEVENV_ROOT"
        version="$1"
        dist="$(mktemp -d)"
        trap 'rm -rf "$dist"' EXIT
        helm package ${chartDir} --version "''${version#v}" --app-version "$version" --destination "$dist"
        helm push "$dist/surrealdb-credential-operator-''${version#v}.tgz" ${chartRepository}
      '';
    };
    publish = {
      description = "Publish the image and chart for a release tag";
      exec = ''
        set -euo pipefail
        publish-image "$1"
        publish-chart "$1"
      '';
    };
  };

  enterTest = "check";
}
