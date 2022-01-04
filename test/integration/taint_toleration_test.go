package integration_test

import (
	"context"
	"fmt"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"k8s.io/apimachinery/pkg/util/rand"
	"open-cluster-management.io/registration/pkg/spoke"
	"open-cluster-management.io/registration/test/integration/util"
	"path"
	"time"
)

var _ = ginkgo.Describe("Use the controller to automatically map cluster status to taints", func() {
	var managedClusterName string
	var hubKubeconfigSecret string
	var hubKubeconfigDir string

	ginkgo.BeforeEach(func() {
		managedClusterName = fmt.Sprintf("managedcluster-%s", rand.String(6))
		hubKubeconfigSecret = fmt.Sprintf("%s-secret", managedClusterName)
		hubKubeconfigDir = path.Join(util.TestDir, "leasetest", fmt.Sprintf("%s-config", managedClusterName))
	})

	//ginkgo.It("Available conditions", func() {
	//	_, err := clusterClient.ClusterV1().ManagedClusters().Create(context.Background(), managedCluster, metav1.CreateOptions{})
	//	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	//
	//	gomega.Eventually(func() bool {
	//		cluster, err := clusterClient.ClusterV1().ManagedClusters().Get(context.Background(), managedCluster.Name, metav1.GetOptions{})
	//		if err != nil {
	//			return false
	//		}
	//		fmt.Printf("cluster--------%+v", cluster)
	//		return true
	//	}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())
	//})

	ginkgo.It("managed cluster lease should be updated constantly", func() {
		// run registration agent
		go func() {
			agentOptions := spoke.SpokeAgentOptions{
				ClusterName:              managedClusterName,
				BootstrapKubeconfig:      bootstrapKubeConfigFile,
				HubKubeconfigSecret:      hubKubeconfigSecret,
				HubKubeconfigDir:         hubKubeconfigDir,
				ClusterHealthCheckPeriod: 1 * time.Minute,
			}
			err := agentOptions.RunSpokeAgent(context.Background(), &controllercmd.ControllerContext{
				KubeConfig:    spokeCfg,
				EventRecorder: util.NewIntegrationTestEventRecorder("cluster-leasetest"),
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		}()

		c, _ := util.GetManagedCluster(clusterClient, managedClusterName)
		fmt.Printf("begin of test then cluster: %+v", c)

		// simulate hub cluster admin to accept the managedcluster and approve the csr
		gomega.Eventually(func() bool {
			if err := util.AcceptManagedCluster(clusterClient, managedClusterName); err != nil {
				return false
			}
			c, _ := util.GetManagedCluster(clusterClient, managedClusterName)
			fmt.Printf("AcceptManagedCluster then cluster: %d", len(c.Spec.Taints))
			return true
		}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())

		//gomega.Eventually(func() bool {
		//	if err := util.ApproveSpokeClusterCSR(kubeClient, managedClusterName, time.Hour*24); err != nil {
		//		return false
		//	}
		//	c, _ := util.GetManagedCluster(clusterClient, managedClusterName)
		//	fmt.Printf("ApproveSpokeClusterCSR then cluster: %+v", c)
		//	return true
		//}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())
		//
		//// simulate k8s to mount the hub kubeconfig secret after the bootstrap is finished
		//gomega.Eventually(func() bool {
		//	if _, err := util.GetFilledHubKubeConfigSecret(kubeClient, testNamespace, hubKubeconfigSecret); err != nil {
		//		return false
		//	}
		//	c, _ := util.GetManagedCluster(clusterClient, managedClusterName)
		//	fmt.Printf("GetFilledHubKubeConfigSecret then cluster: %+v", c)
		//	return true
		//}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())
		//
		//// after two grace period, make sure the managed cluster is available
		//select {
		//case <-time.After(time.Duration(2*5*util.TestLeaseDurationSeconds) * time.Second):
		//	managedCluster, err := util.GetManagedCluster(clusterClient, managedClusterName)
		//	gomega.Expect(err).NotTo(gomega.HaveOccurred())
		//	availableCond := meta.FindStatusCondition(managedCluster.Status.Conditions, clusterv1.ManagedClusterConditionAvailable)
		//	gomega.Expect(availableCond).ShouldNot(gomega.BeNil())
		//	gomega.Expect(availableCond.Status).Should(gomega.Equal(metav1.ConditionTrue))
		//	fmt.Printf("!-----------%+v", managedCluster)
		//}
	})
})
