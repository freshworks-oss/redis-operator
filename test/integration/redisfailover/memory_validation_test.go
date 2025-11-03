//go:build integration
// +build integration

package redisfailover_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/homedir"

	redisfailoverv1 "github.com/freshworks/redis-operator/api/redisfailover/v1"
	"github.com/freshworks/redis-operator/cmd/utils"
	"github.com/freshworks/redis-operator/log"
	"github.com/freshworks/redis-operator/metrics"
	"github.com/freshworks/redis-operator/operator/redisfailover"
	"github.com/freshworks/redis-operator/service/k8s"
	"github.com/freshworks/redis-operator/service/redis"
)

// TestMemoryValidationComprehensive runs comprehensive memory validation tests
func TestMemoryValidationComprehensive(t *testing.T) {
	require := require.New(t)
	currentNamespace := "memory-validation-" + namespace

	// Create signal channels.
	stopC := make(chan struct{})
	errC := make(chan error)
	ctx, cancel := context.WithCancel(context.Background())

	flags := &utils.CMDFlags{
		KubeConfig:  filepath.Join(homedir.HomeDir(), ".kube", "config"),
		Development: true,
	}

	// Kubernetes clients.
	k8sClient, customClient, aeClientset, err := utils.CreateKubernetesClients(flags)
	require.NoError(err)

	// Create the redis clients
	redisClient := redis.New(metrics.Dummy)

	clients := clients{
		k8sClient:   k8sClient,
		rfClient:    customClient,
		aeClient:    aeClientset,
		redisClient: redisClient,
	}

	// Create kubernetes service.
	k8sservice := k8s.New(k8sClient, customClient, aeClientset, log.Dummy, metrics.Dummy)

	// Prepare namespace
	prepErr := clients.prepareNS(currentNamespace)
	require.NoError(prepErr)

	// Give time to the namespace to be ready
	time.Sleep(15 * time.Second)

	// Create operator and run.
	redisfailoverOperator, err := redisfailover.New(redisfailover.Config{}, k8sservice, k8sClient, currentNamespace, redisClient, metrics.Dummy, log.Dummy)
	require.NoError(err)

	go func() {
		errC <- redisfailoverOperator.Run(ctx)
	}()

	// Prepare cleanup for when the test ends
	defer cancel()
	defer clients.cleanup(stopC, currentNamespace)

	// Give time to the operator to start
	time.Sleep(15 * time.Second)

	// Test memory validation for new deployments
	t.Run("Memory Validation New Deployments", func(t *testing.T) {
		clients.testMemoryValidationNewDeployment(t, currentNamespace)
	})

	// Test memory validation for existing deployments
	t.Run("Memory Validation Existing Deployments", func(t *testing.T) {
		clients.testMemoryValidationExistingDeployment(t, currentNamespace)
	})

	// Test memory overhead percentage configuration
	t.Run("Memory Overhead Percentage", func(t *testing.T) {
		clients.testMemoryOverheadPercentage(t, currentNamespace)
	})

	// Test memory validation scenarios
	t.Run("Memory Validation Scenarios", func(t *testing.T) {
		clients.testMemoryValidationScenarios(t, currentNamespace)
	})

	// Test memory parsing edge cases
	t.Run("Memory Parsing Edge Cases", func(t *testing.T) {
		clients.testMemoryParsingEdgeCases(t, currentNamespace)
	})

	// Test memory threshold boundaries
	t.Run("Memory Threshold Boundaries", func(t *testing.T) {
		clients.testMemoryThresholdBoundaries(t, currentNamespace)
	})
}

// testMemoryValidationScenarios tests various memory validation scenarios
func (c *clients) testMemoryValidationScenarios(t *testing.T, currentNamespace string) {
	assert := assert.New(t)
	require := require.New(t)

	testCases := []struct {
		name          string
		podMemory     string
		overhead      int32
		maxmemory     string
		shouldSucceed bool
		description   string
	}{
		{
			name:          "valid-1gb-90percent",
			podMemory:     "1Gi",
			overhead:      10,
			maxmemory:     "900mb",
			shouldSucceed: true,
			description:   "900MB maxmemory with 1GB pod memory and 10% overhead should succeed",
		},
		{
			name:          "invalid-1gb-95percent",
			podMemory:     "1Gi",
			overhead:      10,
			maxmemory:     "950mb",
			shouldSucceed: false,
			description:   "950MB maxmemory with 1GB pod memory and 10% overhead should fail",
		},
		{
			name:          "valid-2gb-80percent",
			podMemory:     "2Gi",
			overhead:      20,
			maxmemory:     "1600mb",
			shouldSucceed: true,
			description:   "1600MB maxmemory with 2GB pod memory and 20% overhead should succeed",
		},
		{
			name:          "invalid-512mb-95percent",
			podMemory:     "512Mi",
			overhead:      5,
			maxmemory:     "500mb",
			shouldSucceed: false,
			description:   "500MB maxmemory with 512MB pod memory and 5% overhead should fail",
		},
		{
			name:          "valid-4gb-75percent",
			podMemory:     "4Gi",
			overhead:      25,
			maxmemory:     "3gb",
			shouldSucceed: true,
			description:   "3GB maxmemory with 4GB pod memory and 25% overhead should succeed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rf := &redisfailoverv1.RedisFailover{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tc.name,
					Namespace: currentNamespace,
				},
				Spec: redisfailoverv1.RedisFailoverSpec{
					Redis: redisfailoverv1.RedisSettings{
						Replicas:                 1,
						MemoryOverheadPercentage: tc.overhead,
						Resources: v1.ResourceRequirements{
							Limits: v1.ResourceList{
								v1.ResourceMemory: resource.MustParse(tc.podMemory),
							},
						},
						CustomConfig: []string{
							fmt.Sprintf("maxmemory %s", tc.maxmemory),
							"maxmemory-policy allkeys-lru",
						},
					},
					Sentinel: redisfailoverv1.SentinelSettings{
						Replicas: 1,
					},
				},
			}

			// Create the RedisFailover
			_, err := c.rfClient.DatabasesV1().RedisFailovers(currentNamespace).Create(context.Background(), rf, metav1.CreateOptions{})
			require.NoError(err, "CRD creation should always succeed")

			// Wait for operator to process
			time.Sleep(15 * time.Second)

			// Check if StatefulSet was created based on expected outcome
			_, err = c.k8sClient.AppsV1().StatefulSets(currentNamespace).Get(context.Background(), fmt.Sprintf("rfr-%s", tc.name), metav1.GetOptions{})

			if tc.shouldSucceed {
				assert.NoError(err, tc.description)
			} else {
				assert.Error(err, tc.description)
			}

			// Cleanup
			err = c.rfClient.DatabasesV1().RedisFailovers(currentNamespace).Delete(context.Background(), tc.name, metav1.DeleteOptions{})
			require.NoError(err)

			// Wait for cleanup
			time.Sleep(5 * time.Second)
		})
	}
}

// testMemoryParsingEdgeCases tests edge cases in memory parsing
func (c *clients) testMemoryParsingEdgeCases(t *testing.T, currentNamespace string) {
	assert := assert.New(t)
	require := require.New(t)

	testCases := []struct {
		name          string
		maxmemory     string
		shouldSucceed bool
		description   string
	}{
		{
			name:          "bytes-format",
			maxmemory:     "536870912", // 512MB in bytes
			shouldSucceed: true,
			description:   "Plain bytes format should be parsed correctly",
		},
		{
			name:          "kb-format",
			maxmemory:     "524288kb", // 512MB in KB
			shouldSucceed: true,
			description:   "KB format should be parsed correctly",
		},
		{
			name:          "mb-format",
			maxmemory:     "512mb",
			shouldSucceed: true,
			description:   "MB format should be parsed correctly",
		},
		{
			name:          "gb-format",
			maxmemory:     "0.5gb",
			shouldSucceed: true,
			description:   "GB format with decimal should be parsed correctly",
		},
		{
			name:          "invalid-format",
			maxmemory:     "invalid-memory",
			shouldSucceed: false,
			description:   "Invalid memory format should be rejected",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rf := &redisfailoverv1.RedisFailover{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tc.name,
					Namespace: currentNamespace,
				},
				Spec: redisfailoverv1.RedisFailoverSpec{
					Redis: redisfailoverv1.RedisSettings{
						Replicas:                 1,
						MemoryOverheadPercentage: 10,
						Resources: v1.ResourceRequirements{
							Limits: v1.ResourceList{
								v1.ResourceMemory: resource.MustParse("1Gi"), // 1GB pod memory
							},
						},
						CustomConfig: []string{
							fmt.Sprintf("maxmemory %s", tc.maxmemory),
							"maxmemory-policy allkeys-lru",
						},
					},
					Sentinel: redisfailoverv1.SentinelSettings{
						Replicas: 1,
					},
				},
			}

			// Create the RedisFailover
			_, err := c.rfClient.DatabasesV1().RedisFailovers(currentNamespace).Create(context.Background(), rf, metav1.CreateOptions{})
			require.NoError(err)

			// Wait for operator to process
			time.Sleep(15 * time.Second)

			// Check StatefulSet creation
			_, err = c.k8sClient.AppsV1().StatefulSets(currentNamespace).Get(context.Background(), fmt.Sprintf("rfr-%s", tc.name), metav1.GetOptions{})

			if tc.shouldSucceed {
				assert.NoError(err, tc.description)
			} else {
				assert.Error(err, tc.description)
			}

			// Cleanup
			err = c.rfClient.DatabasesV1().RedisFailovers(currentNamespace).Delete(context.Background(), tc.name, metav1.DeleteOptions{})
			require.NoError(err)

			time.Sleep(5 * time.Second)
		})
	}
}

// testMemoryThresholdBoundaries tests boundary conditions for memory thresholds
func (c *clients) testMemoryThresholdBoundaries(t *testing.T, currentNamespace string) {
	assert := assert.New(t)
	require := require.New(t)

	// Test Case 1: Exactly at the boundary (should succeed)
	t.Run("Exactly At Boundary", func(t *testing.T) {
		rf := &redisfailoverv1.RedisFailover{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "boundary-exact",
				Namespace: currentNamespace,
			},
			Spec: redisfailoverv1.RedisFailoverSpec{
				Redis: redisfailoverv1.RedisSettings{
					Replicas:                 1,
					MemoryOverheadPercentage: 10, // 90% usable
					Resources: v1.ResourceRequirements{
						Limits: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("1000Mi"), // ~1048MB
						},
					},
					CustomConfig: []string{
						"maxmemory 943mb", // Exactly 90% of ~1048MB
						"maxmemory-policy allkeys-lru",
					},
				},
				Sentinel: redisfailoverv1.SentinelSettings{
					Replicas: 1,
				},
			},
		}

		_, err := c.rfClient.DatabasesV1().RedisFailovers(currentNamespace).Create(context.Background(), rf, metav1.CreateOptions{})
		require.NoError(err)

		time.Sleep(15 * time.Second)

		_, err = c.k8sClient.AppsV1().StatefulSets(currentNamespace).Get(context.Background(), "rfr-boundary-exact", metav1.GetOptions{})
		assert.NoError(err, "Exactly at boundary should succeed")

		// Cleanup
		err = c.rfClient.DatabasesV1().RedisFailovers(currentNamespace).Delete(context.Background(), "boundary-exact", metav1.DeleteOptions{})
		require.NoError(err)
	})

	// Test Case 2: Just over the boundary (should fail)
	t.Run("Just Over Boundary", func(t *testing.T) {
		rf := &redisfailoverv1.RedisFailover{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "boundary-over",
				Namespace: currentNamespace,
			},
			Spec: redisfailoverv1.RedisFailoverSpec{
				Redis: redisfailoverv1.RedisSettings{
					Replicas:                 1,
					MemoryOverheadPercentage: 10, // 90% usable
					Resources: v1.ResourceRequirements{
						Limits: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("1000Mi"), // ~1048MB
						},
					},
					CustomConfig: []string{
						"maxmemory 945mb", // Just over 90% of ~1048MB
						"maxmemory-policy allkeys-lru",
					},
				},
				Sentinel: redisfailoverv1.SentinelSettings{
					Replicas: 1,
				},
			},
		}

		_, err := c.rfClient.DatabasesV1().RedisFailovers(currentNamespace).Create(context.Background(), rf, metav1.CreateOptions{})
		require.NoError(err)

		time.Sleep(15 * time.Second)

		_, err = c.k8sClient.AppsV1().StatefulSets(currentNamespace).Get(context.Background(), "rfr-boundary-over", metav1.GetOptions{})
		assert.Error(err, "Just over boundary should fail")

		// Cleanup
		err = c.rfClient.DatabasesV1().RedisFailovers(currentNamespace).Delete(context.Background(), "boundary-over", metav1.DeleteOptions{})
		require.NoError(err)
	})
}

// testMemoryValidationNewDeployment tests memory validation for new Redis deployments
func (c *clients) testMemoryValidationNewDeployment(t *testing.T, currentNamespace string) {
	assert := assert.New(t)
	require := require.New(t)

	// Test Case 1: New deployment with invalid maxmemory should be blocked
	t.Run("Block Invalid New Deployment", func(t *testing.T) {
		invalidRF := &redisfailoverv1.RedisFailover{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "new-invalid-memory",
				Namespace: currentNamespace,
			},
			Spec: redisfailoverv1.RedisFailoverSpec{
				Redis: redisfailoverv1.RedisSettings{
					Replicas:                 1,
					MemoryOverheadPercentage: 10, // 10% overhead = 90% usable
					Resources: v1.ResourceRequirements{
						Limits: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("1Gi"), // 1GB pod memory
						},
					},
					CustomConfig: []string{
						"maxmemory 1gb", // Invalid: 1GB > 90% of 1GB (922MB)
						"maxmemory-policy allkeys-lru",
					},
				},
				Sentinel: redisfailoverv1.SentinelSettings{
					Replicas: 1,
				},
			},
		}

		// Create the RedisFailover
		_, err := c.rfClient.DatabasesV1().RedisFailovers(currentNamespace).Create(context.Background(), invalidRF, metav1.CreateOptions{})
		require.NoError(err, "CRD creation should succeed")

		// Wait for operator to process
		time.Sleep(20 * time.Second)

		// Verify StatefulSet was NOT created due to validation failure
		_, err = c.k8sClient.AppsV1().StatefulSets(currentNamespace).Get(context.Background(), "rfr-new-invalid-memory", metav1.GetOptions{})
		assert.Error(err, "StatefulSet should NOT be created for invalid memory configuration")

		// Cleanup
		err = c.rfClient.DatabasesV1().RedisFailovers(currentNamespace).Delete(context.Background(), "new-invalid-memory", metav1.DeleteOptions{})
		require.NoError(err)
	})

	// Test Case 2: New deployment with valid maxmemory should succeed
	t.Run("Allow Valid New Deployment", func(t *testing.T) {
		validRF := &redisfailoverv1.RedisFailover{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "new-valid-memory",
				Namespace: currentNamespace,
			},
			Spec: redisfailoverv1.RedisFailoverSpec{
				Redis: redisfailoverv1.RedisSettings{
					Replicas:                 1,
					MemoryOverheadPercentage: 10, // 10% overhead = 90% usable
					Resources: v1.ResourceRequirements{
						Limits: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("2Gi"), // 2GB pod memory
						},
					},
					CustomConfig: []string{
						"maxmemory 1800mb", // Valid: 1800MB < 90% of 2GB (1843MB)
						"maxmemory-policy allkeys-lru",
						"timeout 300",
					},
				},
				Sentinel: redisfailoverv1.SentinelSettings{
					Replicas: 1,
				},
			},
		}

		// Create the RedisFailover
		_, err := c.rfClient.DatabasesV1().RedisFailovers(currentNamespace).Create(context.Background(), validRF, metav1.CreateOptions{})
		require.NoError(err)

		// Wait for StatefulSet creation
		time.Sleep(20 * time.Second)

		// Verify StatefulSet was created successfully
		ss, err := c.k8sClient.AppsV1().StatefulSets(currentNamespace).Get(context.Background(), "rfr-new-valid-memory", metav1.GetOptions{})
		assert.NoError(err, "StatefulSet should be created for valid memory configuration")
		assert.Equal(int32(1), *ss.Spec.Replicas, "StatefulSet should have correct replica count")

		// Cleanup
		err = c.rfClient.DatabasesV1().RedisFailovers(currentNamespace).Delete(context.Background(), "new-valid-memory", metav1.DeleteOptions{})
		require.NoError(err)
	})
}

// testMemoryValidationExistingDeployment tests memory validation for existing Redis deployments
func (c *clients) testMemoryValidationExistingDeployment(t *testing.T, currentNamespace string) {
	assert := assert.New(t)
	require := require.New(t)

	// First, create a valid Redis deployment
	baseRF := &redisfailoverv1.RedisFailover{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "existing-memory-test",
			Namespace: currentNamespace,
		},
		Spec: redisfailoverv1.RedisFailoverSpec{
			Redis: redisfailoverv1.RedisSettings{
				Replicas:                 1,
				MemoryOverheadPercentage: 10,
				Resources: v1.ResourceRequirements{
					Limits: v1.ResourceList{
						v1.ResourceMemory: resource.MustParse("1Gi"),
					},
				},
				CustomConfig: []string{
					"maxmemory 800mb", // Valid initially
					"maxmemory-policy allkeys-lru",
				},
			},
			Sentinel: redisfailoverv1.SentinelSettings{
				Replicas: 1,
			},
		},
	}

	// Create the base RedisFailover
	_, err := c.rfClient.DatabasesV1().RedisFailovers(currentNamespace).Create(context.Background(), baseRF, metav1.CreateOptions{})
	require.NoError(err)

	// Wait for initial deployment
	time.Sleep(30 * time.Second)

	// Verify initial deployment succeeded
	_, err = c.k8sClient.AppsV1().StatefulSets(currentNamespace).Get(context.Background(), "rfr-existing-memory-test", metav1.GetOptions{})
	require.NoError(err, "Initial deployment should succeed")

	// Test Case 1: Update existing deployment with invalid maxmemory
	t.Run("Update Existing With Invalid Memory", func(t *testing.T) {
		// Get the existing RF
		rf, err := c.rfClient.DatabasesV1().RedisFailovers(currentNamespace).Get(context.Background(), "existing-memory-test", metav1.GetOptions{})
		require.NoError(err)

		// Update with invalid maxmemory but valid other configs
		rf.Spec.Redis.CustomConfig = []string{
			"maxmemory 1gb",                 // Invalid: 1GB > 90% of 1GB
			"maxmemory-policy volatile-lru", // Valid: should be applied
			"timeout 600",                   // Valid: should be applied
		}

		// Update the RedisFailover
		_, err = c.rfClient.DatabasesV1().RedisFailovers(currentNamespace).Update(context.Background(), rf, metav1.UpdateOptions{})
		require.NoError(err)

		// Wait for operator to process
		time.Sleep(20 * time.Second)

		// Verify the deployment continues to exist (not blocked)
		ss, err := c.k8sClient.AppsV1().StatefulSets(currentNamespace).Get(context.Background(), "rfr-existing-memory-test", metav1.GetOptions{})
		assert.NoError(err, "Existing deployment should continue to exist even with invalid maxmemory")
		assert.Equal(int32(1), *ss.Spec.Replicas, "StatefulSet should maintain correct replica count")

		// Verify the RF was updated (showing that valid configs were processed)
		updatedRF, err := c.rfClient.DatabasesV1().RedisFailovers(currentNamespace).Get(context.Background(), "existing-memory-test", metav1.GetOptions{})
		require.NoError(err)
		assert.Contains(updatedRF.Spec.Redis.CustomConfig, "maxmemory-policy volatile-lru", "Valid configs should be preserved")
		assert.Contains(updatedRF.Spec.Redis.CustomConfig, "timeout 600", "Valid configs should be preserved")
	})

	// Test Case 2: Update pod memory and maxmemory together
	t.Run("Update Pod Memory And MaxMemory", func(t *testing.T) {
		// Get the existing RF
		rf, err := c.rfClient.DatabasesV1().RedisFailovers(currentNamespace).Get(context.Background(), "existing-memory-test", metav1.GetOptions{})
		require.NoError(err)

		// Update both pod memory and maxmemory
		rf.Spec.Redis.Resources.Limits[v1.ResourceMemory] = resource.MustParse("2Gi") // Increase pod memory to 2GB
		rf.Spec.Redis.CustomConfig = []string{
			"maxmemory 1800mb",             // Valid with new 2GB limit: 1800MB < 90% of 2GB (1843MB)
			"maxmemory-policy allkeys-lfu", // Valid config
		}

		// Update the RedisFailover
		_, err = c.rfClient.DatabasesV1().RedisFailovers(currentNamespace).Update(context.Background(), rf, metav1.UpdateOptions{})
		require.NoError(err)

		// Wait for operator to process the update
		time.Sleep(25 * time.Second)

		// Verify the update was processed
		updatedRF, err := c.rfClient.DatabasesV1().RedisFailovers(currentNamespace).Get(context.Background(), "existing-memory-test", metav1.GetOptions{})
		require.NoError(err)

		// Check that memory limit was updated
		memLimit := updatedRF.Spec.Redis.Resources.Limits[v1.ResourceMemory]
		assert.Equal("2Gi", memLimit.String(), "Pod memory should be updated to 2Gi")

		// Check that configs were updated
		assert.Contains(updatedRF.Spec.Redis.CustomConfig, "maxmemory 1800mb", "Valid maxmemory should be applied")
		assert.Contains(updatedRF.Spec.Redis.CustomConfig, "maxmemory-policy allkeys-lfu", "Valid policy should be applied")
	})

	// Cleanup the test deployment
	err = c.rfClient.DatabasesV1().RedisFailovers(currentNamespace).Delete(context.Background(), "existing-memory-test", metav1.DeleteOptions{})
	require.NoError(err)
}

// testMemoryOverheadPercentage tests different memory overhead percentage configurations
func (c *clients) testMemoryOverheadPercentage(t *testing.T, currentNamespace string) {
	assert := assert.New(t)
	require := require.New(t)

	// Test Case 1: Default overhead percentage (should be 10%)
	t.Run("Default Memory Overhead", func(t *testing.T) {
		defaultRF := &redisfailoverv1.RedisFailover{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "default-overhead-test",
				Namespace: currentNamespace,
			},
			Spec: redisfailoverv1.RedisFailoverSpec{
				Redis: redisfailoverv1.RedisSettings{
					Replicas: 1,
					// MemoryOverheadPercentage not set - should default to 10%
					Resources: v1.ResourceRequirements{
						Limits: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("512Mi"),
						},
					},
					CustomConfig: []string{
						"maxmemory 460mb", // Valid: 460MB < 90% of 512MB (460.8MB)
					},
				},
				Sentinel: redisfailoverv1.SentinelSettings{
					Replicas: 1,
				},
			},
		}

		// Create the RedisFailover
		createdRF, err := c.rfClient.DatabasesV1().RedisFailovers(currentNamespace).Create(context.Background(), defaultRF, metav1.CreateOptions{})
		require.NoError(err)

		// Verify default overhead is applied
		assert.Equal(int32(10), createdRF.Spec.Redis.MemoryOverheadPercentage, "Default memory overhead should be 10%")

		// Wait for StatefulSet creation
		time.Sleep(10 * time.Second)

		// Verify StatefulSet was created
		_, err = c.k8sClient.AppsV1().StatefulSets(currentNamespace).Get(context.Background(), "rfr-default-overhead-test", metav1.GetOptions{})
		assert.NoError(err, "StatefulSet should be created with default overhead")

		// Cleanup
		err = c.rfClient.DatabasesV1().RedisFailovers(currentNamespace).Delete(context.Background(), "default-overhead-test", metav1.DeleteOptions{})
		require.NoError(err)
	})

	// Test Case 2: Custom overhead percentage
	t.Run("Custom Memory Overhead", func(t *testing.T) {
		customRF := &redisfailoverv1.RedisFailover{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "custom-overhead-test",
				Namespace: currentNamespace,
			},
			Spec: redisfailoverv1.RedisFailoverSpec{
				Redis: redisfailoverv1.RedisSettings{
					Replicas:                 1,
					MemoryOverheadPercentage: 25, // 25% overhead = 75% usable
					Resources: v1.ResourceRequirements{
						Limits: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
					CustomConfig: []string{
						"maxmemory 768mb", // Valid: 768MB < 75% of 1GB (768MB)
						"maxmemory-policy allkeys-random",
					},
				},
				Sentinel: redisfailoverv1.SentinelSettings{
					Replicas: 1,
				},
			},
		}

		// Create the RedisFailover
		_, err := c.rfClient.DatabasesV1().RedisFailovers(currentNamespace).Create(context.Background(), customRF, metav1.CreateOptions{})
		require.NoError(err)

		// Wait for StatefulSet creation
		time.Sleep(10 * time.Second)

		// Verify StatefulSet was created
		_, err = c.k8sClient.AppsV1().StatefulSets(currentNamespace).Get(context.Background(), "rfr-custom-overhead-test", metav1.GetOptions{})
		assert.NoError(err, "StatefulSet should be created with custom overhead")

		// Cleanup
		err = c.rfClient.DatabasesV1().RedisFailovers(currentNamespace).Delete(context.Background(), "custom-overhead-test", metav1.DeleteOptions{})
		require.NoError(err)
	})

	// Test Case 3: Edge case - very high overhead percentage
	t.Run("High Memory Overhead", func(t *testing.T) {
		highOverheadRF := &redisfailoverv1.RedisFailover{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "high-overhead-test",
				Namespace: currentNamespace,
			},
			Spec: redisfailoverv1.RedisFailoverSpec{
				Redis: redisfailoverv1.RedisSettings{
					Replicas:                 1,
					MemoryOverheadPercentage: 50, // 50% overhead = 50% usable
					Resources: v1.ResourceRequirements{
						Limits: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("2Gi"),
						},
					},
					CustomConfig: []string{
						"maxmemory 1gb", // Valid: 1GB = 50% of 2GB
						"maxmemory-policy volatile-ttl",
					},
				},
				Sentinel: redisfailoverv1.SentinelSettings{
					Replicas: 1,
				},
			},
		}

		// Create the RedisFailover
		_, err := c.rfClient.DatabasesV1().RedisFailovers(currentNamespace).Create(context.Background(), highOverheadRF, metav1.CreateOptions{})
		require.NoError(err)

		// Wait for StatefulSet creation
		time.Sleep(10 * time.Second)

		// Verify StatefulSet was created
		_, err = c.k8sClient.AppsV1().StatefulSets(currentNamespace).Get(context.Background(), "rfr-high-overhead-test", metav1.GetOptions{})
		assert.NoError(err, "StatefulSet should be created with high overhead")

		// Cleanup
		err = c.rfClient.DatabasesV1().RedisFailovers(currentNamespace).Delete(context.Background(), "high-overhead-test", metav1.DeleteOptions{})
		require.NoError(err)
	})
}
