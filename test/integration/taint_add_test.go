package integration_test

import (
	"context"
	"fmt"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	v1 "open-cluster-management.io/api/cluster/v1"
	"open-cluster-management.io/registration/pkg/spoke"
	"open-cluster-management.io/registration/test/integration/util"
	"path"
	"time"
)

var _ = ginkgo.Describe("ManagedCluster Taints Update", func() {
	var managedClusterName string
	var hubKubeconfigSecret string
	var hubKubeconfigDir string

	ginkgo.BeforeEach(func() {
		managedClusterName = fmt.Sprintf("managedcluster-%s", rand.String(6))
		hubKubeconfigSecret = fmt.Sprintf("%s-secret", managedClusterName)
		hubKubeconfigDir = path.Join(util.TestDir, "leasetest", fmt.Sprintf("%s-config", managedClusterName))
	})

	ginkgo.FIt("ManagedCluster taints should be updated automatically", func() {
		ctx, stop := context.WithCancel(context.Background())
		// run registration agent
		go func() {
			agentOptions := spoke.SpokeAgentOptions{
				ClusterName:              managedClusterName,
				BootstrapKubeconfig:      bootstrapKubeConfigFile,
				HubKubeconfigSecret:      hubKubeconfigSecret,
				HubKubeconfigDir:         hubKubeconfigDir,
				ClusterHealthCheckPeriod: 1 * time.Minute,
			}
			err := agentOptions.RunSpokeAgent(ctx, &controllercmd.ControllerContext{
				KubeConfig:    spokeCfg,
				EventRecorder: util.NewIntegrationTestEventRecorder("cluster-leasetest"),
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		}()

		gomega.Eventually(func() bool {
			if err := util.AcceptManagedCluster(clusterClient, managedClusterName); err != nil {
				return false
			}
			return true
		}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())

		gomega.Eventually(func() bool {
			managedCluster, err := util.GetManagedCluster(clusterClient, managedClusterName)
			if err != nil {
				return false
			}
			if len(managedCluster.Spec.Taints) != 1 {
				return false
			}
			if managedCluster.Spec.Taints[0] != (v1.Taint{
				Key:       "cluster.open-cluster-management.io/unreachable",
				Value:     "NoManagedClusterConditionAvailable",
				Effect:    "NoSelect",
				TimeAdded: managedCluster.Spec.Taints[0].TimeAdded,
			}) {
				return false
			}
			return true
		}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())

		gomega.Eventually(func() bool {
			if err := util.ApproveSpokeClusterCSR(kubeClient, managedClusterName, time.Hour*24); err != nil {
				return false
			}
			return true
		}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())

		// The managed cluster is available, so taints is expected to be empty
		gomega.Eventually(func() bool {
			managedCluster, err := util.GetManagedCluster(clusterClient, managedClusterName)
			if err != nil {
				return false
			}
			if len(managedCluster.Spec.Taints) != 0 {
				return false
			}
			return true
		}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())

		// make sure the managed available condition is cluster unknown
		stop()
		gomega.Eventually(func() bool {
			managedCluster, err := util.GetManagedCluster(clusterClient, managedClusterName)
			if err != nil {
				return false
			}
			if len(managedCluster.Spec.Taints) != 1 {
				return false
			}
			if managedCluster.Spec.Taints[0] != (v1.Taint{
				Key:       "cluster.open-cluster-management.io/unreachable",
				Value:     "ManagedClusterConditionAvailableUnknown",
				Effect:    "NoSelect",
				TimeAdded: managedCluster.Spec.Taints[0].TimeAdded,
			}) {
				return false
			}
			return true
		}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())

		// Change available condition status to false
		managedCluster, err := clusterClient.ClusterV1().ManagedClusters().Get(context.Background(), managedClusterName, metav1.GetOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		condition := meta.FindStatusCondition(managedCluster.Status.Conditions, v1.ManagedClusterConditionAvailable)
		condition.Status = "False"
		meta.SetStatusCondition(&(managedCluster.Status.Conditions), *condition)
		_, err = clusterClient.ClusterV1().ManagedClusters().UpdateStatus(context.Background(), managedCluster, metav1.UpdateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		gomega.Eventually(func() bool {
			managedCluster, err := util.GetManagedCluster(clusterClient, managedClusterName)
			if err != nil {
				return false
			}
			if len(managedCluster.Spec.Taints) != 1 {
				return false
			}
			if managedCluster.Spec.Taints[0] != (v1.Taint{
				Key:       "cluster.open-cluster-management.io/unavailable",
				Value:     "ManagedClusterConditionAvailableFalse",
				Effect:    "NoSelect",
				TimeAdded: managedCluster.Spec.Taints[0].TimeAdded,
			}) {
				return false
			}
			return true
		}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())
	})
})
