// Dagger workflows for SurrealDB Credential Operator.
package main

import (
	"context"
	"fmt"
	"strings"

	"dagger/surrealdb-credential-operator/internal/dagger"
)

const (
	workspace    = "/workspace"
	defaultImage = "ghcr.io/seamlezz/surrealdb-credential-operator"
	defaultChart = "oci://ghcr.io/seamlezz/charts"
	chartDir     = "charts/surrealdb-credential-operator"
)

var publishPlatforms = []dagger.Platform{"linux/amd64", "linux/arm64"}

type SurrealdbCredentialOperator struct {
	Source *dagger.Directory `json:"source"`
}

func New(
	// +defaultPath="/"
	// +ignore=[".git", ".dagger", "bin", "cover.out", "dist"]
	source *dagger.Directory,
) *SurrealdbCredentialOperator {
	return &SurrealdbCredentialOperator{Source: source}
}

func (m *SurrealdbCredentialOperator) Check(ctx context.Context) error {
	if err := m.Lint(ctx); err != nil {
		return err
	}
	if err := m.Test(ctx); err != nil {
		return err
	}
	if err := m.Manifests(ctx); err != nil {
		return err
	}
	return m.Chart(ctx)
}

// +check
func (m *SurrealdbCredentialOperator) Lint(ctx context.Context) error {
	_, err := m.goBase().WithExec([]string{"go", "vet", "./..."}).Sync(ctx)
	return err
}

// +check
func (m *SurrealdbCredentialOperator) Test(ctx context.Context) error {
	_, err := m.goBase().WithExec([]string{"make", "test"}).Sync(ctx)
	return err
}

// +check
func (m *SurrealdbCredentialOperator) Manifests(ctx context.Context) error {
	_, err := m.goBase().
		WithExec([]string{"make", "manifests"}).
		Sync(ctx)
	return err
}

// +check
func (m *SurrealdbCredentialOperator) Chart(ctx context.Context) error {
	_, err := dag.Container().From("alpine/helm:3.17.0").
		WithDirectory(workspace, m.Source).
		WithWorkdir(workspace).
		WithExec([]string{"helm", "lint", chartDir}).
		WithExec([]string{"helm", "template", "surrealdb-credential-operator", chartDir}).
		Sync(ctx)
	return err
}

func (m *SurrealdbCredentialOperator) E2e(ctx context.Context) error {
	image, err := m.BuildImage(ctx, "")
	if err != nil {
		return err
	}
	if _, err := image.Sync(ctx); err != nil {
		return err
	}

	k3s := dag.K3S("surrealdb-credential-operator-e2e")
	server, err := k3s.Server().Start(ctx)
	if err != nil {
		return err
	}
	_, _ = server.Endpoint(ctx)
	_, err = m.goBase().
		WithServiceBinding("kubernetes", server).
		WithFile("/root/.kube/config", k3s.Config()).
		WithEnvVariable("KUBECONFIG", "/root/.kube/config").
		WithEnvVariable("E2E_PROJECT_DIR", workspace).
		WithEnvVariable("E2E_LOCAL_MANAGER", "true").
		WithExec([]string{"go", "test", "-tags=e2e", "./test/e2e", "-v", "-count=1", "-timeout=12m"}).
		Sync(ctx)
	return err
}

func (m *SurrealdbCredentialOperator) BuildImage(ctx context.Context,
	// +optional
	platform dagger.Platform,
) (*dagger.Container, error) {
	if platform == "" {
		var err error
		platform, err = dag.DefaultPlatform(ctx)
		if err != nil {
			return nil, err
		}
	}

	goos, goarch, err := goTarget(platform)
	if err != nil {
		return nil, err
	}

	binary := dag.Container().From("golang:1.24").
		WithDirectory(workspace, m.Source).
		WithWorkdir(workspace).
		WithEnvVariable("CGO_ENABLED", "0").
		WithEnvVariable("GOOS", goos).
		WithEnvVariable("GOARCH", goarch).
		WithExec([]string{"go", "mod", "download"}).
		WithExec([]string{"go", "build", "-a", "-o", "manager", "cmd/main.go"}).
		File("/workspace/manager")

	return dag.Container(dagger.ContainerOpts{Platform: platform}).
		From("gcr.io/distroless/static:nonroot").
		WithWorkdir("/").
		WithFile("/manager", binary).
		WithUser("65532:65532").
		WithEntrypoint([]string{"/manager"}), nil
}

func goTarget(platform dagger.Platform) (string, string, error) {
	goos, goarch, ok := strings.Cut(string(platform), "/")
	if !ok || goos == "" || goarch == "" {
		return "", "", fmt.Errorf("invalid platform %q", platform)
	}
	return goos, goarch, nil
}

func (m *SurrealdbCredentialOperator) PublishImage(ctx context.Context,
	// +optional
	// +default="ghcr.io/seamlezz/surrealdb-credential-operator"
	image string,
	tag string,
	username string,
	password *dagger.Secret,
) (string, error) {
	variants := make([]*dagger.Container, 0, len(publishPlatforms))
	for _, platform := range publishPlatforms {
		ctr, err := m.BuildImage(ctx, platform)
		if err != nil {
			return "", err
		}
		variants = append(variants, ctr)
	}
	return dag.Container().WithRegistryAuth("ghcr.io", username, password).
		Publish(ctx, fmt.Sprintf("%s:%s", image, tag), dagger.ContainerPublishOpts{PlatformVariants: variants})
}

func (m *SurrealdbCredentialOperator) PackageChart(ctx context.Context, version string, appVersion string) (*dagger.File, error) {
	out := dag.Container().From("alpine/helm:3.17.0").
		WithDirectory(workspace, m.Source).
		WithWorkdir(workspace).
		WithExec([]string{"sh", "-c", fmt.Sprintf("helm package %s --version %s --app-version %s --destination /dist", chartDir, strings.TrimPrefix(version, "v"), appVersion)})
	filename := fmt.Sprintf("/dist/surrealdb-credential-operator-%s.tgz", strings.TrimPrefix(version, "v"))
	return out.File(filename), nil
}

func (m *SurrealdbCredentialOperator) PublishChart(ctx context.Context,
	version string,
	username string,
	password *dagger.Secret,
	// +optional
	// +default="oci://ghcr.io/seamlezz/charts"
	repository string,
) (string, error) {
	chart, err := m.PackageChart(ctx, version, version)
	if err != nil {
		return "", err
	}
	out, err := dag.Container().From("alpine/helm:3.17.0").
		WithMountedFile("/tmp/chart.tgz", chart).
		WithSecretVariable("HELM_PASSWORD", password).
		WithEnvVariable("HELM_USERNAME", username).
		WithExec([]string{"sh", "-c", "helm registry login ghcr.io --username \"$HELM_USERNAME\" --password \"$HELM_PASSWORD\""}).
		WithExec([]string{"helm", "push", "/tmp/chart.tgz", repository}).
		Stdout(ctx)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (m *SurrealdbCredentialOperator) Publish(ctx context.Context,
	tag string,
	username string,
	password *dagger.Secret,
) (string, error) {
	imageRef, err := m.PublishImage(ctx, defaultImage, tag, username, password)
	if err != nil {
		return "", err
	}
	chartRef, err := m.PublishChart(ctx, tag, username, password, defaultChart)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("image=%s\nchart=%s", imageRef, chartRef), nil
}

func (m *SurrealdbCredentialOperator) goBase() *dagger.Container {
	return dag.Container().From("golang:1.26-alpine").
		WithExec([]string{"apk", "add", "--no-cache", "bash", "curl", "git", "make", "kubectl"}).
		WithDirectory(workspace, m.Source).
		WithWorkdir(workspace).
		WithMountedCache("/go/pkg/mod", dag.CacheVolume("surrealdb-credential-operator-go-mod")).
		WithMountedCache("/root/.cache/go-build", dag.CacheVolume("surrealdb-credential-operator-go-build"))
}
