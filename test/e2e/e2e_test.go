//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	surrealdb "github.com/surrealdb/surrealdb.go"
)

const (
	operatorNamespace = "surrealdb-credential-operator-system"
	surrealNamespace  = "surrealdb-e2e"
	appNamespace      = "smoke-e2e"
)

var (
	managerCmd     *exec.Cmd
	portForwardCmd *exec.Cmd
)

var _ = Describe("SurrealDB credential lifecycle", Ordered, func() {
	BeforeAll(func() {
		if img := os.Getenv("E2E_OPERATOR_IMAGE"); img != "" {
			projectImage = img
		}
		waitForKubernetes()
		run("make", "install")
		if os.Getenv("E2E_LOCAL_MANAGER") == "true" {
			startLocalManager()
		} else {
			run("make", "deploy", "IMG="+projectImage)
			wait("deployment/surrealdb-credential-operator-controller-manager", operatorNamespace)
		}
		installSurrealDB()
		startSurrealPortForward()
		bootstrapSurrealNamespaceAndDatabase()
	})

	AfterAll(func() {
		stopSurrealPortForward()
		runAllowFail("kubectl", "delete", "ns", appNamespace, "--ignore-not-found", "--wait=false")
		runAllowFail("kubectl", "delete", "ns", surrealNamespace, "--ignore-not-found", "--wait=false")
		stopLocalManager()
		if os.Getenv("E2E_LOCAL_MANAGER") != "true" {
			runAllowFail("make", "undeploy")
		}
		runAllowFail("kubectl", "delete", "crd",
			"surrealdbcredentials.surrealdb.seamlezz.com",
			"surrealdbproviders.surrealdb.seamlezz.com",
			"surrealdbtenantpolicies.surrealdb.seamlezz.com",
			"--ignore-not-found", "--wait=false")
	})

	It("creates, rotates, and removes a database credential", func() {
		run("kubectl", "create", "ns", appNamespace)
		apply(fmt.Sprintf(`
apiVersion: surrealdb.seamlezz.com/v1alpha1
kind: SurrealDBProvider
metadata:
  name: e2e
spec:
  endpoint: %s
  rootCredentialRef:
    namespace: %s
    name: surrealdb-root
`, providerEndpoint(), surrealNamespace))
		apply(fmt.Sprintf(`
apiVersion: surrealdb.seamlezz.com/v1alpha1
kind: SurrealDBTenantPolicy
metadata:
  name: smoke
  namespace: %s
spec:
  providerRef:
    name: e2e
  surrealNamespace: smoke
  databaseUsers:
    allowedDatabases: [smoke]
    allowedRoles: [VIEWER, EDITOR]
`, appNamespace))
		apply(fmt.Sprintf(`
apiVersion: surrealdb.seamlezz.com/v1alpha1
kind: SurrealDBCredential
metadata:
  name: smoke-editor
  namespace: %s
spec:
  policyRef:
    name: smoke
  level: Database
  database: smoke
  roles: [EDITOR]
  targetSecret:
    name: surrealdb-smoke-credentials
`, appNamespace))

		run("kubectl", "wait", "surrealdbprovider/e2e", "--for=condition=Ready", "--timeout=120s")
		run("kubectl", "wait", "surrealdbtenantpolicy/smoke", "-n", appNamespace, "--for=condition=Ready", "--timeout=120s")
		run("kubectl", "wait", "surrealdbcredential/smoke-editor", "-n", appNamespace, "--for=condition=Ready", "--timeout=120s")

		Eventually(func(g Gomega) {
			out, err := command("kubectl", "get", "secret", "surrealdb-smoke-credentials", "-n", appNamespace, "-o", "jsonpath={.data.username}").CombinedOutput()
			g.Expect(err).NotTo(HaveOccurred(), "%s\n%s", string(out), diagnostics())
			g.Expect(strings.TrimSpace(string(out))).NotTo(BeEmpty(), diagnostics())
		}, 2*time.Minute, 2*time.Second).Should(Succeed())

		username := secretValue(appNamespace, "surrealdb-smoke-credentials", "username")
		password := secretValue(appNamespace, "surrealdb-smoke-credentials", "password")
		Expect(username).NotTo(BeEmpty())
		Expect(password).NotTo(BeEmpty())
		Expect(secretValue(appNamespace, "surrealdb-smoke-credentials", "url")).NotTo(BeEmpty())
		Expect(secretValue(appNamespace, "surrealdb-smoke-credentials", "namespace")).To(Equal("smoke"))
		Expect(secretValue(appNamespace, "surrealdb-smoke-credentials", "database")).To(Equal("smoke"))
		Expect(secretValue(appNamespace, "surrealdb-smoke-credentials", "level")).To(Equal("database"))
		Expect(secretValue(appNamespace, "surrealdb-smoke-credentials", "SURREAL_USER")).To(Equal(username))
		Expect(secretValue(appNamespace, "surrealdb-smoke-credentials", "SURREAL_PASS")).To(Equal(password))
		expectSurrealSignIn(username, password, true)

		patch := fmt.Sprintf(`{"metadata":{"annotations":{"surrealdb.seamlezz.com/rotate-at":"%s"}}}`, time.Now().UTC().Format(time.RFC3339Nano))
		run("kubectl", "patch", "surrealdbcredential", "smoke-editor", "-n", appNamespace, "--type=merge", "-p", patch)
		Eventually(func(g Gomega) {
			g.Expect(secretValue(appNamespace, "surrealdb-smoke-credentials", "password")).NotTo(Equal(password))
		}, 2*time.Minute, 2*time.Second).Should(Succeed())
		rotatedPassword := secretValue(appNamespace, "surrealdb-smoke-credentials", "password")
		Expect(secretValue(appNamespace, "surrealdb-smoke-credentials", "SURREAL_PASS")).To(Equal(rotatedPassword))
		expectSurrealSignIn(username, password, false)
		expectSurrealSignIn(username, rotatedPassword, true)

		run("kubectl", "delete", "surrealdbcredential", "smoke-editor", "-n", appNamespace)
		Eventually(func(g Gomega) {
			_, err := command("kubectl", "get", "secret", "surrealdb-smoke-credentials", "-n", appNamespace).CombinedOutput()
			g.Expect(err).To(HaveOccurred())
		}, 2*time.Minute, 2*time.Second).Should(Succeed())
		expectSurrealSignIn(username, rotatedPassword, false)
	})

	It("denies disallowed database roles without creating a Secret", func() {
		apply(fmt.Sprintf(`
apiVersion: surrealdb.seamlezz.com/v1alpha1
kind: SurrealDBCredential
metadata:
  name: smoke-owner-denied
  namespace: %s
spec:
  policyRef:
    name: smoke
  level: Database
  database: smoke
  roles: [OWNER]
  targetSecret:
    name: surrealdb-owner-denied
`, appNamespace))

		Eventually(func(g Gomega) {
			out, err := command("kubectl", "get", "surrealdbcredential", "smoke-owner-denied", "-n", appNamespace, "-o", "jsonpath={.status.conditions[?(@.type=='Ready')].reason}").CombinedOutput()
			g.Expect(err).NotTo(HaveOccurred(), string(out))
			g.Expect(strings.TrimSpace(string(out))).To(Equal("PolicyDenied"))
		}, 2*time.Minute, 2*time.Second).Should(Succeed())

		_, err := command("kubectl", "get", "secret", "surrealdb-owner-denied", "-n", appNamespace).CombinedOutput()
		Expect(err).To(HaveOccurred())
	})
})

func installSurrealDB() {
	run("kubectl", "create", "ns", surrealNamespace)
	apply(fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: surrealdb-root
  namespace: %s
type: Opaque
stringData:
  username: root
  password: rootpass
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: surrealdb
  namespace: %s
spec:
  replicas: 1
  selector:
    matchLabels:
      app: surrealdb
  template:
    metadata:
      labels:
        app: surrealdb
    spec:
      containers:
      - name: surrealdb
        image: surrealdb/surrealdb:v3.1.2
        args: ["start", "--bind", "0.0.0.0:8000", "--user", "root", "--pass", "rootpass", "memory"]
        ports:
        - containerPort: 8000
---
apiVersion: v1
kind: Service
metadata:
  name: surrealdb
  namespace: %s
spec:
  selector:
    app: surrealdb
  ports:
  - name: http
    port: 8000
    targetPort: 8000
`, surrealNamespace, surrealNamespace, surrealNamespace))
	wait("deployment/surrealdb", surrealNamespace)
}

func bootstrapSurrealNamespaceAndDatabase() {
	// SurrealDBCredential deliberately does not create namespaces/databases.
	// Create them through the root user before requesting app credentials.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := surrealdb.FromEndpointURLString(ctx, localSurrealEndpoint())
	Expect(err).NotTo(HaveOccurred())
	defer func() { _ = db.Close(context.Background()) }()
	_, err = db.SignIn(ctx, surrealdb.Auth{Username: "root", Password: "rootpass"})
	Expect(err).NotTo(HaveOccurred())
	results, err := surrealdb.Query[any](ctx, db, "DEFINE NAMESPACE smoke; USE NS smoke; DEFINE DATABASE smoke;", nil)
	Expect(err).NotTo(HaveOccurred())
	for i, result := range *results {
		Expect(result.Error).NotTo(HaveOccurred(), "SurrealQL bootstrap statement %d failed", i)
	}
}

func expectSurrealSignIn(username, password string, shouldSucceed bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := surrealdb.FromEndpointURLString(ctx, localSurrealEndpoint())
	Expect(err).NotTo(HaveOccurred())
	defer func() { _ = db.Close(context.Background()) }()

	_, err = db.SignIn(ctx, surrealdb.Auth{Namespace: "smoke", Database: "smoke", Username: username, Password: password})
	if shouldSucceed {
		Expect(err).NotTo(HaveOccurred())
		results, err := surrealdb.Query[any](ctx, db, "RETURN true;", nil)
		Expect(err).NotTo(HaveOccurred())
		for i, result := range *results {
			Expect(result.Error).NotTo(HaveOccurred(), "generated credential query %d failed", i)
		}
		return
	}
	Expect(err).To(HaveOccurred())
}

func startSurrealPortForward() {
	portForwardCmd = command("kubectl", "port-forward", "-n", surrealNamespace, "svc/surrealdb", "18000:8000")
	portForwardCmd.Stdout = GinkgoWriter
	portForwardCmd.Stderr = GinkgoWriter
	portForwardCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	Expect(portForwardCmd.Start()).To(Succeed())
	DeferCleanup(stopSurrealPortForward)
	Eventually(func(g Gomega) {
		resp, err := http.Get(localSurrealHTTPEndpoint() + "/health")
		g.Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		g.Expect(resp.StatusCode).To(BeNumerically(">=", 200))
		g.Expect(resp.StatusCode).To(BeNumerically("<", 300))
	}, time.Minute, time.Second).Should(Succeed())
}

func stopSurrealPortForward() {
	if portForwardCmd == nil || portForwardCmd.Process == nil {
		return
	}
	_ = syscall.Kill(-portForwardCmd.Process.Pid, syscall.SIGTERM)
	_, _ = portForwardCmd.Process.Wait()
	portForwardCmd = nil
}

func providerEndpoint() string {
	if os.Getenv("E2E_LOCAL_MANAGER") == "true" {
		return localSurrealEndpoint()
	}
	return fmt.Sprintf("ws://surrealdb.%s.svc.cluster.local:8000", surrealNamespace)
}

func localSurrealEndpoint() string {
	return "ws://127.0.0.1:18000"
}

func localSurrealHTTPEndpoint() string {
	return "http://127.0.0.1:18000"
}

func waitForKubernetes() {
	Eventually(func(g Gomega) {
		out, err := command("kubectl", "version").CombinedOutput()
		g.Expect(err).NotTo(HaveOccurred(), string(out))
	}, 2*time.Minute, 2*time.Second).Should(Succeed())
}

func startLocalManager() {
	managerCmd = command("go", "run", "./cmd/main.go", "--metrics-bind-address=0", "--health-probe-bind-address=0")
	managerCmd.Stdout = GinkgoWriter
	managerCmd.Stderr = GinkgoWriter
	managerCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	Expect(managerCmd.Start()).To(Succeed())
	DeferCleanup(stopLocalManager)
}

func stopLocalManager() {
	if managerCmd == nil || managerCmd.Process == nil {
		return
	}
	_ = syscall.Kill(-managerCmd.Process.Pid, syscall.SIGTERM)
	_, _ = managerCmd.Process.Wait()
	managerCmd = nil
}

func apply(yaml string) {
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(yaml)
	_, err := cmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred())
}

func wait(resource, namespace string) {
	run("kubectl", "rollout", "status", resource, "-n", namespace, "--timeout=180s")
}

func secretValue(namespace, name, key string) string {
	encoded := runOut("kubectl", "get", "secret", name, "-n", namespace, "-o", fmt.Sprintf("jsonpath={.data.%s}", key))
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	Expect(err).NotTo(HaveOccurred())
	return string(decoded)
}

func diagnostics() string {
	commands := [][]string{
		{"kubectl", "get", "surrealdbprovider", "e2e", "-o", "yaml"},
		{"kubectl", "get", "surrealdbtenantpolicy", "smoke", "-n", appNamespace, "-o", "yaml"},
		{"kubectl", "get", "surrealdbcredential", "smoke-editor", "-n", appNamespace, "-o", "yaml"},
		{"kubectl", "get", "events", "-n", appNamespace, "--sort-by=.lastTimestamp"},
	}
	var b strings.Builder
	for _, args := range commands {
		b.WriteString("\n$ ")
		b.WriteString(strings.Join(args, " "))
		b.WriteString("\n")
		out, err := command(args[0], args[1:]...).CombinedOutput()
		b.Write(out)
		if err != nil {
			b.WriteString(err.Error())
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func run(name string, args ...string) {
	out, err := command(name, args...).CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "%s\n%s", strings.Join(append([]string{name}, args...), " "), string(out))
}

func runInput(input, name string, args ...string) {
	cmd := command(name, args...)
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "%s\n%s", strings.Join(append([]string{name}, args...), " "), string(out))
}

func runOut(name string, args ...string) string {
	out, err := command(name, args...).CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), string(out))
	return strings.TrimSpace(string(out))
}

func runAllowFail(name string, args ...string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = projectRoot()
	cmd.Env = append(os.Environ(), "GO111MODULE=on")
	_, _ = cmd.CombinedOutput()
}

func command(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.Dir = projectRoot()
	cmd.Env = append(os.Environ(), "GO111MODULE=on")
	return cmd
}

func projectRoot() string {
	if root := os.Getenv("E2E_PROJECT_DIR"); root != "" {
		return root
	}
	wd, err := os.Getwd()
	Expect(err).NotTo(HaveOccurred())
	return strings.TrimSuffix(wd, "/test/e2e")
}
