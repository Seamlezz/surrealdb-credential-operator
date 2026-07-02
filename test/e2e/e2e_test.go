//go:build e2e
// +build e2e

package e2e

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	operatorNamespace = "surrealdb-credential-operator-system"
	surrealNamespace  = "surrealdb-e2e"
	appNamespace      = "smoke-e2e"
)

var _ = Describe("SurrealDB credential lifecycle", Ordered, func() {
	BeforeAll(func() {
		if img := os.Getenv("E2E_OPERATOR_IMAGE"); img != "" {
			projectImage = img
		}
		run("make", "install")
		run("make", "deploy", "IMG="+projectImage)
		wait("deployment/surrealdb-credential-operator-controller-manager", operatorNamespace)
		installSurrealDB()
		bootstrapSurrealNamespaceAndDatabase()
	})

	AfterAll(func() {
		runAllowFail("kubectl", "delete", "ns", appNamespace, "--ignore-not-found")
		runAllowFail("kubectl", "delete", "ns", surrealNamespace, "--ignore-not-found")
		runAllowFail("make", "undeploy")
		runAllowFail("make", "uninstall")
	})

	It("creates, rotates, and removes a database credential", func() {
		run("kubectl", "create", "ns", appNamespace)
		apply(fmt.Sprintf(`
apiVersion: surrealdb.seamlezz.com/v1alpha1
kind: SurrealDBProvider
metadata:
  name: e2e
spec:
  endpoint: http://surrealdb.%s.svc.cluster.local:8000
  rootCredentialRef:
    namespace: %s
    name: surrealdb-root
`, surrealNamespace, surrealNamespace))
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

		Eventually(func(g Gomega) {
			out := runOut("kubectl", "get", "secret", "surrealdb-smoke-credentials", "-n", appNamespace, "-o", "jsonpath={.data.username}")
			g.Expect(out).NotTo(BeEmpty())
		}, 2*time.Minute, 2*time.Second).Should(Succeed())

		username := secretValue(appNamespace, "surrealdb-smoke-credentials", "username")
		password := secretValue(appNamespace, "surrealdb-smoke-credentials", "password")
		Expect(username).NotTo(BeEmpty())
		Expect(password).NotTo(BeEmpty())
		Expect(secretValue(appNamespace, "surrealdb-smoke-credentials", "SURREAL_USER")).To(Equal(username))
		Expect(secretValue(appNamespace, "surrealdb-smoke-credentials", "SURREAL_PASS")).To(Equal(password))

		patch := fmt.Sprintf(`{"metadata":{"annotations":{"surrealdb.seamlezz.com/rotate-at":"%s"}}}`, time.Now().UTC().Format(time.RFC3339Nano))
		run("kubectl", "patch", "surrealdbcredential", "smoke-editor", "-n", appNamespace, "--type=merge", "-p", patch)
		Eventually(func(g Gomega) {
			g.Expect(secretValue(appNamespace, "surrealdb-smoke-credentials", "password")).NotTo(Equal(password))
		}, 2*time.Minute, 2*time.Second).Should(Succeed())

		run("kubectl", "delete", "surrealdbcredential", "smoke-editor", "-n", appNamespace)
		Eventually(func(g Gomega) {
			_, err := exec.Command("kubectl", "get", "secret", "surrealdb-smoke-credentials", "-n", appNamespace).CombinedOutput()
			g.Expect(err).To(HaveOccurred())
		}, 2*time.Minute, 2*time.Second).Should(Succeed())
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
	run("kubectl", "run", "surreal-bootstrap", "-n", surrealNamespace, "--restart=Never", "--rm", "-i", "--image=surrealdb/surrealdb:v3.1.2", "--", "sql", "--endpoint", "http://surrealdb:8000", "--username", "root", "--password", "rootpass", "--hide-welcome", "--pretty", "DEFINE NAMESPACE smoke; USE NS smoke; DEFINE DATABASE smoke;")
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

func run(name string, args ...string) {
	_, err := command(name, args...).CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), strings.Join(append([]string{name}, args...), " "))
}

func runOut(name string, args ...string) string {
	out, err := command(name, args...).CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), string(out))
	return strings.TrimSpace(string(out))
}

func runAllowFail(name string, args ...string) {
	_, _ = command(name, args...).CombinedOutput()
}

func command(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), "GO111MODULE=on")
	return cmd
}
