//go:build integration
// +build integration

package redisfailover_test

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	rediscli "github.com/go-redis/redis/v8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/util/homedir"

	redisfailoverv1 "github.com/freshworks/redis-operator/api/redisfailover/v1"
	"github.com/freshworks/redis-operator/cmd/utils"
	"github.com/freshworks/redis-operator/log"
	"github.com/freshworks/redis-operator/metrics"
	"github.com/freshworks/redis-operator/operator/redisfailover"
	"github.com/freshworks/redis-operator/service/k8s"
	"github.com/freshworks/redis-operator/service/redis"
)

const (
	standaloneNamespace = "standalone-testns"
	standaloneRFName    = "standalone-test"

	bootstrapNamespace = "standalone-bootstrap-testns"
	bootstrapSrcName   = "bootstrap-src"
	bootstrapDestName  = "bootstrap-dest"
	bootstrapTestKey   = "bootstrap-test-key"
	bootstrapTestValue = "hello-from-source"
)

// getRedisPods lists the Redis pods for rfName via the StatefulSet's own selector,
// same pattern as testRedisMaster/testSentinelMonitoring, rather than guessing at label
// keys directly.
func (c *clients) getRedisPods(currentNamespace, rfName string) (*corev1.PodList, error) {
	redisSS, err := c.k8sClient.AppsV1().StatefulSets(currentNamespace).Get(context.Background(), fmt.Sprintf("rfr-%s", rfName), metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return c.k8sClient.CoreV1().Pods(currentNamespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: labels.FormatLabels(redisSS.Spec.Selector.MatchLabels),
	})
}

// TestRedisFailoverStandalone exercises spec.standalone=true end-to-end against a real
// cluster: exactly one Redis pod comes up, it self-promotes to master, and none of the
// Sentinel-HA-only resources (Sentinel Deployment, slave Service, PDB, shutdown
// ConfigMap) get created for it.
func TestRedisFailoverStandalone(t *testing.T) {
	req := require.New(t)
	currentNamespace := standaloneNamespace

	stopC := make(chan struct{})
	errC := make(chan error)
	ctx, cancel := context.WithCancel(context.Background())

	flags := &utils.CMDFlags{
		KubeConfig:  filepath.Join(homedir.HomeDir(), ".kube", "config"),
		Development: true,
	}

	k8sClient, customClient, aeClientset, err := utils.CreateKubernetesClients(flags)
	req.NoError(err)

	redisClient := redis.New(metrics.Dummy)

	clients := clients{
		k8sClient:   k8sClient,
		rfClient:    customClient,
		aeClient:    aeClientset,
		redisClient: redisClient,
	}

	k8sservice := k8s.New(k8sClient, customClient, aeClientset, log.Dummy, metrics.Dummy)

	req.NoError(clients.prepareNS(currentNamespace))
	time.Sleep(15 * time.Second)

	redisfailoverOperator, err := redisfailover.New(redisfailover.Config{OperatorGroupID: integrationTestGroupID}, k8sservice, k8sClient, currentNamespace, redisClient, metrics.Dummy, log.Dummy)
	req.NoError(err)

	go func() {
		errC <- redisfailoverOperator.Run(ctx)
	}()

	defer cancel()
	defer clients.cleanup(stopC, currentNamespace)

	time.Sleep(15 * time.Second)

	toCreate := &redisfailoverv1.RedisFailover{
		ObjectMeta: metav1.ObjectMeta{
			Name:      standaloneRFName,
			Namespace: currentNamespace,
			Labels:    map[string]string{rfOperatorGroupLabelKey: integrationTestGroupID},
		},
		Spec: redisfailoverv1.RedisFailoverSpec{
			Standalone: true,
		},
	}
	_, err = clients.rfClient.DatabasesV1().RedisFailovers(currentNamespace).Create(context.Background(), toCreate, metav1.CreateOptions{})
	req.NoError(err)

	var redisPods *corev1.PodList
	req.Eventually(func() bool {
		ss, err := clients.k8sClient.AppsV1().StatefulSets(currentNamespace).Get(context.Background(), fmt.Sprintf("rfr-%s", standaloneRFName), metav1.GetOptions{})
		if err != nil || ss.Status.ReadyReplicas != 1 {
			return false
		}
		pods, err := clients.getRedisPods(currentNamespace, standaloneRFName)
		if err != nil || len(pods.Items) != 1 {
			return false
		}
		redisPods = pods
		return true
	}, 4*time.Minute, 5*time.Second, "standalone redis pod never became ready")

	t.Run("StatefulSet has exactly 1 replica", func(t *testing.T) {
		require := require.New(t)
		ss, err := clients.k8sClient.AppsV1().StatefulSets(currentNamespace).Get(context.Background(), fmt.Sprintf("rfr-%s", standaloneRFName), metav1.GetOptions{})
		require.NoError(err)
		assert.Equal(t, int32(1), *ss.Spec.Replicas)
	})

	t.Run("Pod is master", func(t *testing.T) {
		require := require.New(t)
		require.Len(redisPods.Items, 1)
		ip := redisPods.Items[0].Status.PodIP
		isMaster, err := clients.redisClient.IsMaster(ip, "6379", "")
		require.NoError(err)
		assert.True(t, isMaster, "the single standalone pod should be master")
	})

	t.Run("Pod reports Ready", func(t *testing.T) {
		require := require.New(t)
		require.Len(redisPods.Items, 1)
		ready := false
		for _, cond := range redisPods.Items[0].Status.Conditions {
			if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
				ready = true
			}
		}
		assert.True(t, ready, "standalone pod should pass its readiness probe")
	})

	t.Run("No Sentinel Deployment", func(t *testing.T) {
		_, err := clients.k8sClient.AppsV1().Deployments(currentNamespace).Get(context.Background(), fmt.Sprintf("rfs-%s", standaloneRFName), metav1.GetOptions{})
		assert.True(t, errors.IsNotFound(err), "standalone should never create a Sentinel Deployment")
	})

	t.Run("No slave Service", func(t *testing.T) {
		_, err := clients.k8sClient.CoreV1().Services(currentNamespace).Get(context.Background(), fmt.Sprintf("rfrs-%s", standaloneRFName), metav1.GetOptions{})
		assert.True(t, errors.IsNotFound(err), "standalone should never create a slave Service")
	})

	t.Run("No shutdown ConfigMap", func(t *testing.T) {
		_, err := clients.k8sClient.CoreV1().ConfigMaps(currentNamespace).Get(context.Background(), fmt.Sprintf("rfr-s-%s", standaloneRFName), metav1.GetOptions{})
		assert.True(t, errors.IsNotFound(err), "standalone should never create a shutdown ConfigMap")
	})

	t.Run("No PodDisruptionBudget", func(t *testing.T) {
		_, err := clients.k8sClient.PolicyV1().PodDisruptionBudgets(currentNamespace).Get(context.Background(), fmt.Sprintf("rfr-%s", standaloneRFName), metav1.GetOptions{})
		assert.True(t, errors.IsNotFound(err), "standalone should never create a PodDisruptionBudget")
	})

	t.Run("Master Service exists", func(t *testing.T) {
		require := require.New(t)
		svc, err := clients.k8sClient.CoreV1().Services(currentNamespace).Get(context.Background(), fmt.Sprintf("rfrm-%s", standaloneRFName), metav1.GetOptions{})
		require.NoError(err)
		assert.NotEmpty(t, svc.Spec.ClusterIP)
	})
}

// TestRedisFailoverStandaloneBootstrap exercises the standalone+bootstrapNode migration
// path end-to-end: a destination standalone pod replicates real data from a source
// standalone instance, then — once bootstrapNode is removed from the spec — promotes
// itself to master without losing the data it already replicated.
func TestRedisFailoverStandaloneBootstrap(t *testing.T) {
	req := require.New(t)
	currentNamespace := bootstrapNamespace

	stopC := make(chan struct{})
	errC := make(chan error)
	ctx, cancel := context.WithCancel(context.Background())

	flags := &utils.CMDFlags{
		KubeConfig:  filepath.Join(homedir.HomeDir(), ".kube", "config"),
		Development: true,
	}

	k8sClient, customClient, aeClientset, err := utils.CreateKubernetesClients(flags)
	req.NoError(err)

	redisClient := redis.New(metrics.Dummy)

	clients := clients{
		k8sClient:   k8sClient,
		rfClient:    customClient,
		aeClient:    aeClientset,
		redisClient: redisClient,
	}

	k8sservice := k8s.New(k8sClient, customClient, aeClientset, log.Dummy, metrics.Dummy)

	req.NoError(clients.prepareNS(currentNamespace))
	time.Sleep(15 * time.Second)

	redisfailoverOperator, err := redisfailover.New(redisfailover.Config{OperatorGroupID: integrationTestGroupID}, k8sservice, k8sClient, currentNamespace, redisClient, metrics.Dummy, log.Dummy)
	req.NoError(err)

	go func() {
		errC <- redisfailoverOperator.Run(ctx)
	}()

	defer cancel()
	defer clients.cleanup(stopC, currentNamespace)

	time.Sleep(15 * time.Second)

	// Source: a plain standalone instance, no bootstrapNode.
	src := &redisfailoverv1.RedisFailover{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bootstrapSrcName,
			Namespace: currentNamespace,
			Labels:    map[string]string{rfOperatorGroupLabelKey: integrationTestGroupID},
		},
		Spec: redisfailoverv1.RedisFailoverSpec{
			Standalone: true,
		},
	}
	_, err = clients.rfClient.DatabasesV1().RedisFailovers(currentNamespace).Create(context.Background(), src, metav1.CreateOptions{})
	req.NoError(err)

	var srcPodIP string
	req.Eventually(func() bool {
		pods, err := clients.getRedisPods(currentNamespace, bootstrapSrcName)
		if err != nil || len(pods.Items) != 1 || pods.Items[0].Status.PodIP == "" {
			return false
		}
		isMaster, err := clients.redisClient.IsMaster(pods.Items[0].Status.PodIP, "6379", "")
		if err != nil || !isMaster {
			return false
		}
		srcPodIP = pods.Items[0].Status.PodIP
		return true
	}, 4*time.Minute, 5*time.Second, "source standalone pod never became master")

	// Write a real key on the source so we can prove it actually got replicated, not
	// just that a connection was established.
	srcClient := rediscli.NewClient(&rediscli.Options{Addr: net.JoinHostPort(srcPodIP, "6379"), DB: 0})
	defer srcClient.Close()
	req.NoError(srcClient.Set(context.Background(), bootstrapTestKey, bootstrapTestValue, 0).Err())

	srcMasterSvc, err := clients.k8sClient.CoreV1().Services(currentNamespace).Get(context.Background(), fmt.Sprintf("rfrm-%s", bootstrapSrcName), metav1.GetOptions{})
	req.NoError(err)
	req.NotEmpty(srcMasterSvc.Spec.ClusterIP)

	// Destination: standalone, seeded from the source via bootstrapNode.
	dest := &redisfailoverv1.RedisFailover{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bootstrapDestName,
			Namespace: currentNamespace,
			Labels:    map[string]string{rfOperatorGroupLabelKey: integrationTestGroupID},
		},
		Spec: redisfailoverv1.RedisFailoverSpec{
			Standalone: true,
			BootstrapNode: &redisfailoverv1.BootstrapSettings{
				Host: srcMasterSvc.Spec.ClusterIP,
				Port: "6379",
			},
		},
	}
	_, err = clients.rfClient.DatabasesV1().RedisFailovers(currentNamespace).Create(context.Background(), dest, metav1.CreateOptions{})
	req.NoError(err)

	var destPodIP string
	t.Run("Destination replicates real data from the source", func(t *testing.T) {
		require := require.New(t)
		require.Eventually(func() bool {
			pods, err := clients.getRedisPods(currentNamespace, bootstrapDestName)
			if err != nil || len(pods.Items) != 1 || pods.Items[0].Status.PodIP == "" {
				return false
			}
			ip := pods.Items[0].Status.PodIP
			c := rediscli.NewClient(&rediscli.Options{Addr: net.JoinHostPort(ip, "6379"), DB: 0})
			defer c.Close()
			val, err := c.Get(context.Background(), bootstrapTestKey).Result()
			if err != nil || val != bootstrapTestValue {
				return false
			}
			destPodIP = ip
			return true
		}, 4*time.Minute, 5*time.Second, "destination never caught up with the source's data")

		isMaster, err := clients.redisClient.IsMaster(destPodIP, "6379", "")
		require.NoError(err)
		assert.False(t, isMaster, "destination should still be a replica while bootstrapNode is set")
	})

	// Cut over: remove bootstrapNode so the destination promotes itself.
	gotDest, err := clients.rfClient.DatabasesV1().RedisFailovers(currentNamespace).Get(context.Background(), bootstrapDestName, metav1.GetOptions{})
	req.NoError(err)
	gotDest.Spec.BootstrapNode = nil
	_, err = clients.rfClient.DatabasesV1().RedisFailovers(currentNamespace).Update(context.Background(), gotDest, metav1.UpdateOptions{})
	req.NoError(err)

	t.Run("Destination promotes to master after bootstrapNode is removed, without losing data", func(t *testing.T) {
		require := require.New(t)
		require.Eventually(func() bool {
			isMaster, err := clients.redisClient.IsMaster(destPodIP, "6379", "")
			return err == nil && isMaster
		}, 3*time.Minute, 5*time.Second, "destination never promoted to master after removing bootstrapNode")

		c := rediscli.NewClient(&rediscli.Options{Addr: net.JoinHostPort(destPodIP, "6379"), DB: 0})
		defer c.Close()
		val, err := c.Get(context.Background(), bootstrapTestKey).Result()
		require.NoError(err)
		assert.Equal(t, bootstrapTestValue, val, "promoted destination should still have the data it replicated")
	})
}
