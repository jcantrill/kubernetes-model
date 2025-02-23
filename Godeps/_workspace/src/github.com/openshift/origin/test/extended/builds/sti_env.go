package builds

import (
	"fmt"
	"path/filepath"
	"strings"

	"k8s.io/kubernetes/test/e2e"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	buildapi "github.com/openshift/origin/pkg/build/api"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("default: STI build with .sti/environment file", func() {
	defer g.GinkgoRecover()
	var (
		imageStreamFixture = exutil.FixturePath("..", "integration", "fixtures", "test-image-stream.json")
		stiEnvBuildFixture = exutil.FixturePath("fixtures", "test-env-build.json")
		oc                 = exutil.NewCLI("build-sti-env", exutil.KubeConfigPath())
	)

	g.Describe("Building from a template", func() {
		g.It(fmt.Sprintf("should create a image from %q template and run it in a pod", stiEnvBuildFixture), func() {
			oc.SetOutputDir(exutil.TestContext.OutputDir)

			g.By(fmt.Sprintf("calling oc create -f %q", imageStreamFixture))
			err := oc.Run("create").Args("-f", imageStreamFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("calling oc create -f %q", stiEnvBuildFixture))
			err = oc.Run("create").Args("-f", stiEnvBuildFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting a test build")
			buildName, err := oc.Run("start-build").Args("test").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("expecting the build is in Complete phase")
			err = exutil.WaitForABuild(oc.REST().Builds(oc.Namespace()), buildName,
				// The build passed
				func(b *buildapi.Build) bool {
					return b.Name == buildName && b.Status.Phase == buildapi.BuildPhaseComplete
				},
				// The build failed
				func(b *buildapi.Build) bool {
					if b.Name != buildName {
						return false
					}
					return b.Status.Phase == buildapi.BuildPhaseFailed || b.Status.Phase == buildapi.BuildPhaseError
				},
			)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("getting the Docker image reference from ImageStream")
			imageName, err := exutil.GetDockerImageReference(oc.REST().ImageStreams(oc.Namespace()), "test", "latest")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("writing the pod defintion to a file")
			outputPath := filepath.Join(exutil.TestContext.OutputDir, oc.Namespace()+"-sample-pod.json")
			pod := exutil.CreatePodForImage(imageName)
			err = exutil.WriteObjectToFile(pod, outputPath)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("calling oc create -f %q", outputPath))
			err = oc.Run("create").Args("-f", outputPath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("expecting the pod to be running")
			err = oc.KubeFramework().WaitForPodRunning(pod.Name)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("expecting the pod container has TEST_ENV variable set")
			out, err := oc.Run("exec").Args("-p", pod.Name, "--", "curl", "http://0.0.0.0:8080").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			if !strings.Contains(out, "success") {
				e2e.Failf("Pod %q contains does not contain expected variable: %q", pod.Name, out)
			}
		})
	})
})
