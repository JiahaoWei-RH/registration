package taints

import (
	"context"
	v1 "open-cluster-management.io/api/cluster/v1"
	"testing"
	"time"

	clusterfake "open-cluster-management.io/api/client/cluster/clientset/versioned/fake"
	clusterinformers "open-cluster-management.io/api/client/cluster/informers/externalversions"
	testinghelpers "open-cluster-management.io/registration/pkg/helpers/testing"

	"github.com/openshift/library-go/pkg/operator/events/eventstesting"

	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

func TestSyncTaintCluster(t *testing.T) {
	cases := []struct {
		name            string
		startingObjects []runtime.Object
		validateActions func(t *testing.T, actions []clienttesting.Action)
	}{
		{
			name:            "managed cluster is available",
			startingObjects: []runtime.Object{testinghelpers.NewAvailableManagedCluster()},
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				testinghelpers.AssertActions(t, actions, "update")
				managedCluster := (actions[0].(clienttesting.UpdateActionImpl).Object).(*v1.ManagedCluster)
				testinghelpers.AssertTaints(t, managedCluster.Spec.Taints, []v1.Taint{})
			},
		},
		{
			name:            "managed cluster is unavailable",
			startingObjects: []runtime.Object{testinghelpers.NewUnAvailableManagedCluster()},
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				testinghelpers.AssertActions(t, actions, "update")
				managedCluster := (actions[0].(clienttesting.UpdateActionImpl).Object).(*v1.ManagedCluster)
				taints := []v1.Taint{
					{
						Key:    v1.ManagedClusterTaintUnavailable,
						Value:  ConditionFalse,
						Effect: v1.TaintEffectNoSelect,
						// Ignore the assertion of time
						TimeAdded: managedCluster.Spec.Taints[0].TimeAdded,
					},
				}
				testinghelpers.AssertTaints(t, managedCluster.Spec.Taints, taints)
			},
		},
		{
			name:            "managed cluster is unreachable",
			startingObjects: []runtime.Object{testinghelpers.NewManagedCluster()},
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				testinghelpers.AssertActions(t, actions, "update")
				managedCluster := (actions[0].(clienttesting.UpdateActionImpl).Object).(*v1.ManagedCluster)
				taints := []v1.Taint{
					{
						Key:       v1.ManagedClusterTaintUnreachable,
						Value:     NoCondition,
						Effect:    v1.TaintEffectNoSelect,
						TimeAdded: managedCluster.Spec.Taints[0].TimeAdded,
					},
				}
				testinghelpers.AssertTaints(t, managedCluster.Spec.Taints, taints)
			},
		},
		{
			name:            "managed cluster is unreachable",
			startingObjects: []runtime.Object{testinghelpers.NewUnknownManagedCluster()},
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				testinghelpers.AssertActions(t, actions, "update")
				managedCluster := (actions[0].(clienttesting.UpdateActionImpl).Object).(*v1.ManagedCluster)
				taints := []v1.Taint{
					{
						Key:       v1.ManagedClusterTaintUnreachable,
						Value:     ConditionUnknown,
						Effect:    v1.TaintEffectNoSelect,
						TimeAdded: managedCluster.Spec.Taints[0].TimeAdded,
					},
				}
				testinghelpers.AssertTaints(t, managedCluster.Spec.Taints, taints)
			},
		},
		{
			name:            "sync a deleted spoke cluster",
			startingObjects: []runtime.Object{},
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				testinghelpers.AssertNoActions(t, actions)
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			clusterClient := clusterfake.NewSimpleClientset(c.startingObjects...)
			kubeClient := kubefake.NewSimpleClientset()
			clusterInformerFactory := clusterinformers.NewSharedInformerFactory(clusterClient, time.Minute*10)
			clusterStore := clusterInformerFactory.Cluster().V1().ManagedClusters().Informer().GetStore()
			for _, cluster := range c.startingObjects {
				clusterStore.Add(cluster)
			}

			ctrl := taintsController{kubeClient, clusterClient, clusterInformerFactory.Cluster().V1().ManagedClusters().Lister(), eventstesting.NewTestingEventRecorder(t)}
			syncErr := ctrl.sync(context.TODO(), testinghelpers.NewFakeSyncContext(t, testinghelpers.TestManagedClusterName))
			if syncErr != nil {
				t.Errorf("unexpected err: %v", syncErr)
			}

			c.validateActions(t, clusterClient.Actions())
		})
	}
}
