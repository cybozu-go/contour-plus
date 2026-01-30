package e2e

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	stableEnvKey = "STABLE"
)

type HTTPProxyBuidler struct {
	httpProxies map[string]struct{}
}

func NewHTTPProxyBuilder() *HTTPProxyBuidler {
	return &HTTPProxyBuidler{httpProxies: make(map[string]struct{})}
}

// NewHTTPProxy creates HTTPProxy and returns its name
func (h *HTTPProxyBuidler) NewHTTPProxy(g Gomega) string {
	name := fmt.Sprintf("helloworld-%v", len(h.httpProxies))
	stdout, stderr, err := runCommand("sed", nil, fmt.Sprintf("s/NAME/%s/g", name), "testdata/httpproxy.yaml")
	g.Expect(err).NotTo(HaveOccurred(), "stdout: %s, stderr: %s", stdout, stderr)
	kubectlSafe(g, stdout, "apply", "-f", "-")
	if h.httpProxies == nil {
		h.httpProxies = make(map[string]struct{})
	}
	h.httpProxies[name] = struct{}{}
	return name
}

// CleanUp deletes all HTTPProxies created
func (h *HTTPProxyBuidler) CleanUp(g Gomega) {
	for name := range h.httpProxies {
		kubectlSafe(g, nil, "delete", "httpproxies", name)
	}
}

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test")
}

var _ = BeforeSuite(func() {
	SetDefaultEventuallyPollingInterval(time.Second)
	SetDefaultEventuallyTimeout(5 * time.Minute)
})

var _ = Describe("Test contour-plus", func() {
	runTest()
})

func runTest() {
	It("should deploy contour-plus and enable TLS from outside of the cluster", func() {
		g := NewWithT(GinkgoT())
		b := NewHTTPProxyBuilder()
		defer b.CleanUp(g)
		// create httpd first
		kubectlSafe(g, nil, "apply", "-f", "testdata/httpd.yaml")
		defer kubectlSafe(g, nil, "delete", "-f", "testdata/httpd.yaml")
		kubectlSafe(g, nil, "rollout", "status", "deployment/httpd-backend", "--timeout=5m")

		By("creating first httpproxy")
		first := b.NewHTTPProxy(g)

		By("deploying contour-plus")
		assignNodeIP(g)
		applyContourPlus(g, "ghcr.io/cybozu-go/contour-plus:dev")
		defer deleteContourPlus(g)

		By("checking TLS readiness of first httpproxy")
		testTLS(g, first)

		By("restarting contour-plus")
		restartContourPlus(g)
		By("checking TLS readiness of first httpproxy after controller restart")
		testTLS(g, first)

		By("creating second httpproxy")
		second := b.NewHTTPProxy(g)
		By("checking TLS readiness of second httpproxy")
		testTLS(g, second)

		By("checking if there is an infinite loop")
		checkForInfiniteLoop(g, first)
		checkForInfiniteLoop(g, second)
	})

	It("should upgrade contour-plus and enable TLS from outside of the cluster", func() {
		g := NewWithT(GinkgoT())
		b := NewHTTPProxyBuilder()
		defer b.CleanUp(g)
		// create httpd first
		kubectlSafe(g, nil, "apply", "-f", "testdata/httpd.yaml")
		defer kubectlSafe(g, nil, "delete", "-f", "testdata/httpd.yaml")
		kubectlSafe(g, nil, "rollout", "status", "deployment/httpd-backend", "--timeout=5m")

		By("creating first httpproxy")
		first := b.NewHTTPProxy(g)

		By("deploying latest contour-plus release")
		assignNodeIP(g)
		applyContourPlus(g, fmt.Sprintf("ghcr.io/cybozu-go/contour-plus:%s", getLatestStableReleaseTag()))
		defer deleteContourPlus(g)

		By("checking TLS readiness of first httpproxy")
		testTLS(g, first)

		By("upgrading contour-plus to the image built from this branch")
		applyContourPlus(g, "ghcr.io/cybozu-go/contour-plus:dev")
		By("checking TLS readiness of first httpproxy after the upgrade")
		testTLS(g, first)

		By("creating second httpproxy")
		second := b.NewHTTPProxy(g)
		By("checking TLS readiness of second httpproxy")
		testTLS(g, second)

		time.Sleep(10 * time.Second)
		By("checking if there is an infinite loop")
		checkForInfiniteLoop(g, first)
		checkForInfiniteLoop(g, second)
	})
}

func assignNodeIP(g Gomega) {
	// Get IP of contour-plus-e2e-worker container on the Docker network
	out := dockerSafe(g, nil,
		"inspect",
		"-f", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}",
		"contour-plus-e2e-worker",
	)
	ip := strings.TrimSpace(string(out))
	g.Expect(ip).NotTo(BeEmpty(), "node IP must not be empty")

	patch := fmt.Sprintf(`{"status":{"loadBalancer":{"ingress":[{"ip":"%s","ipMode":"VIP"}]}}}`, ip)

	// patch envoy load balancer external-ip
	kubectlSafe(g, nil,
		"-n", "projectcontour",
		"patch", "svc", "envoy",
		"--subresource=status",
		"--type=merge",
		"-p", patch,
	)
}

func applyContourPlus(g Gomega, image string) {
	kout := kustomizeSafe(g, nil, "build", "testdata/contour-plus")
	yqExpr := fmt.Sprintf(`
	  (. |
	    select(.kind == "Deployment" and .metadata.name == "contour-plus") |
	    .spec.template.spec.containers[]
	    | select(.name == "contour-plus")
	    | .image
	  ) = "%s"
	`, image)
	yout := yqSafe(g, kout, "eval", yqExpr, "-")
	kubectlSafe(g, yout, "apply", "-f", "-")
	kubectlSafe(g, nil, "rollout", "status", "-n", "projectcontour", "deploy/contour-plus", "--timeout=5m")
}

func restartContourPlus(g Gomega) {
	kubectlSafe(g, nil, "rollout", "restart", "-n", "projectcontour", "deploy/contour-plus")
	kubectlSafe(g, nil, "rollout", "status", "-n", "projectcontour", "deploy/contour-plus", "--timeout=5m")
}

func deleteContourPlus(g Gomega) {
	kout := kustomizeSafe(g, nil, "build", "testdata/contour-plus")
	kubectlSafe(g, kout, "delete", "-f", "-")
	kubectlSafe(g, nil, "wait", "--for=delete", "deploy/contour-plus", "--timeout=10s")
}

func testTLS(g Gomega, name string) {
	clientName := "external-client"
	clientImage := "ghcr.io/cybozu/ubuntu-debug:24.04"
	secretName := fmt.Sprintf("%s-tls", name)
	caFileLocal := fmt.Sprintf("%s-tls.crt", name)
	caFileInContainer := fmt.Sprintf("/usr/local/share/ca-certificates/%s", caFileLocal)
	dnsServer := "contour-plus-e2e-worker"
	dnsPort := "30530"
	host := fmt.Sprintf("%s.default.example.org", name)
	httpsPort := "30443"

	// Start external-client container (detached)
	dockerSafe(g, nil,
		"run",
		"--rm",
		"--network", "kind",
		"--name", clientName,
		"-d",
		clientImage,
		"tail", "-f", "/dev/null",
	)
	defer func() {
		// Clean up container
		dockerSafe(g, nil, "kill", clientName)
	}()

	// Get CA cert from helloworld-tls secret and write to local file
	type secretData struct {
		Data map[string]string `json:"data"`
	}

	var secretJSON []byte
	Eventually(func() []byte {
		var err error
		secretJSON, _, err = kubectl(nil,
			"get", "secret", secretName,
			"-o", "json",
		)
		if err != nil {
			return []byte{}
		}
		return secretJSON
	}, 30*time.Second).NotTo(BeEmpty(), "secret not created.")

	var s secretData
	g.Expect(json.Unmarshal(secretJSON, &s)).To(Succeed())

	caB64, ok := s.Data["ca.crt"]
	g.Expect(ok).To(BeTrue(), "secret %s must contain data[\"ca.crt\"]", secretName)

	caBytes, err := base64.StdEncoding.DecodeString(caB64)
	g.Expect(err).NotTo(HaveOccurred())

	err = os.WriteFile(caFileLocal, caBytes, 0o644)
	g.Expect(err).NotTo(HaveOccurred())
	defer os.Remove(caFileLocal)

	// copy CA cert from local file to the client container and reload.
	dockerSafe(g, nil,
		"cp",
		caFileLocal,
		fmt.Sprintf("%s:%s", clientName, caFileInContainer),
	)
	dockerSafe(g, nil,
		"exec", clientName,
		"update-ca-certificates",
	)

	var vip string
	Eventually(func() string {
		// test that DNS look up succeeds on the hostname
		vipBytes := dockerSafe(g, nil,
			"exec", clientName,
			"dig",
			"@"+dnsServer,
			"-p", dnsPort,
			"-4",
			"+short",
			host,
		)
		vip = strings.TrimSpace(string(vipBytes))
		return vip
	}, 90*time.Second).NotTo(BeEmpty(), "VIP from dig must not be empty")

	// use the override resolved IP for the DNS lookup by curl since we are using
	// our own dns server that is not running on the standard dns port.
	resolveArg := fmt.Sprintf("%s:%s:%s", host, httpsPort, vip)
	valBytes := dockerSafe(g, nil,
		"exec", clientName,
		"curl",
		"--fail",
		"--silent",
		"--show-error",
		"--resolve", resolveArg,
		fmt.Sprintf("https://%s:%s", host, httpsPort),
	)
	val := strings.TrimSpace(string(valBytes))

	fmt.Fprintf(GinkgoWriter, "TLS response body: %q\n", val)
	g.Expect(val).To(Equal("Hello"), "response body from TLS endpoint should not be empty")
}

func getLatestStableReleaseTag() string {
	val, ok := os.LookupEnv(stableEnvKey)
	if !ok {
		return "latest"
	}
	return val
}

func checkForInfiniteLoop(g Gomega, certName string) {
	By("waiting for Certificate to become ready")
	Eventually(func() bool {
		out, _, err := kubectl(nil, "get", "certificate", certName, "-o", "yaml")
		if err != nil {
			return false
		}
		stdout := yqSafe(g, out, `.status.conditions[] | select(.type == "Ready") | .status`)
		return strings.TrimSpace(string(stdout)) == "True"
	}, 30*time.Second).Should(BeTrue(), "certificate did not become Ready in time")

	By("checking resourceVersion of Certificate")
	initial := snapshotCertRV(g, certName)
	Consistently(func() string {
		return snapshotCertRV(g, certName)
	}, 10*time.Second, 1*time.Second).Should(Equal(initial), "possible infinite loop detected")

}

func snapshotCertRV(g Gomega, name string) string {
	out := kubectlSafe(g, nil, "get", "certificate", name, "-o", "yaml")
	stdout := yqSafe(g, out, `.metadata.resourceVersion`)
	return strings.TrimSpace(string(stdout))
}
