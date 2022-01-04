package taints

import (
	"context"
	"fmt"
	"time"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	clientset "open-cluster-management.io/api/client/cluster/clientset/versioned"
	informerv1 "open-cluster-management.io/api/client/cluster/informers/externalversions/cluster/v1"
	listerv1 "open-cluster-management.io/api/client/cluster/listers/cluster/v1"
	v1 "open-cluster-management.io/api/cluster/v1"
)

// taintsController
type taintsController struct {
	kubeClient    kubernetes.Interface
	clusterClient clientset.Interface
	clusterLister listerv1.ManagedClusterLister
	eventRecorder events.Recorder
}

// NewTaintsController creates a new taints controller
func NewTaintsController(
	kubeClient kubernetes.Interface,
	clusterClient clientset.Interface,
	clusterInformer informerv1.ManagedClusterInformer,
	recorder events.Recorder) factory.Controller {
	c := &taintsController{
		kubeClient:    kubeClient,
		clusterClient: clusterClient,
		clusterLister: clusterInformer.Lister(),
		eventRecorder: recorder.WithComponentSuffix("taints-controller"),
	}
	return factory.New().
		WithInformersQueueKeyFunc(func(obj runtime.Object) string {
			accessor, _ := meta.Accessor(obj)
			return accessor.GetName()
		}, clusterInformer.Informer()).
		WithSync(c.sync).
		ToController("taintsController", recorder)
}

func (c *taintsController) sync(ctx context.Context, syncCtx factory.SyncContext) error {
	managedClusterName := syncCtx.QueueKey()
	klog.V(4).Infof("Reconciling Taints %s", managedClusterName)
	managedCluster, err := c.clusterLister.Get(managedClusterName)
	if errors.IsNotFound(err) {
		// Spoke cluster not found, could have been deleted, do nothing.
		return nil
	}
	if err != nil {
		return err
	}

	managedCluster = managedCluster.DeepCopy()
	fmt.Printf("taints controller begin %+v", managedCluster)

	newTaints := managedCluster.Spec.Taints
	if meta.IsStatusConditionTrue(managedCluster.Status.Conditions, v1.ManagedClusterConditionAvailable) {
		fmt.Println("taints ManagedClusterConditionAvailable")
		newTaints, _ = c.deleteTaintAndJudgeExist(newTaints, "",
			v1.ManagedClusterTaintUnreachable, v1.ManagedClusterTaintUnavailable)
	} else if meta.IsStatusConditionFalse(managedCluster.Status.Conditions, v1.ManagedClusterConditionAvailable) {
		var exist bool
		fmt.Println("taints ManagedClusterTaintUnreachable")
		newTaints, exist = c.deleteTaintAndJudgeExist(newTaints, v1.ManagedClusterTaintUnavailable, v1.ManagedClusterTaintUnreachable)
		if !exist {
			t1 := v1.Taint{
				Key:    v1.ManagedClusterTaintUnavailable,
				Value:  "",
				Effect: v1.TaintEffectNoSelect,
				TimeAdded: metav1.Time{
					Time: time.Now(),
				},
			}
			newTaints = append(newTaints, t1)
		}
	} else {
		var exist bool
		fmt.Println("taints ManagedClusterTaintUnavailable")
		newTaints, exist = c.deleteTaintAndJudgeExist(newTaints, v1.ManagedClusterTaintUnreachable, v1.ManagedClusterTaintUnavailable)
		if !exist {
			t1 := v1.Taint{
				Key:    v1.ManagedClusterTaintUnreachable,
				Value:  "",
				Effect: v1.TaintEffectNoSelect,
				TimeAdded: metav1.Time{
					Time: time.Now(),
				},
			}
			newTaints = append(newTaints, t1)
		}
	}

	managedCluster.Spec.Taints = newTaints
	fmt.Printf("taints controller before update---- %+v", managedCluster)
	managedCluster, err = c.clusterClient.ClusterV1().ManagedClusters().Update(ctx, managedCluster, metav1.UpdateOptions{})
	if err != nil {
		fmt.Printf("err %+v", err)
	}
	k, err := c.clusterLister.Get(managedClusterName)
	if err != nil {
		fmt.Printf("-----------------err %+v", err)
	}
	fmt.Printf("taints controller update %+v", k)
	return err
}

func (c *taintsController) deleteTaintAndJudgeExist(taints []v1.Taint, isExistKey string, deleteKeys ...string) ([]v1.Taint, bool) {
	ans := make([]v1.Taint, 0)
	exist := false

	for _, v := range taints {
		flag := false
		for _, k := range deleteKeys {
			if k == v.Key {
				flag = true
				break
			}
		}
		if !flag {
			ans = append(ans, v)
		}
		if v.Key == isExistKey {
			exist = true
		}
	}

	return ans, exist
}
