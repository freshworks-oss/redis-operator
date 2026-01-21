package service_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/freshworks/redis-operator/log"
	"github.com/freshworks/redis-operator/metrics"
	mK8SService "github.com/freshworks/redis-operator/mocks/service/k8s"
	mRedisService "github.com/freshworks/redis-operator/mocks/service/redis"
	rfservice "github.com/freshworks/redis-operator/operator/redisfailover/service"
)

func TestGetPodDNSName(t *testing.T) {
	tests := []struct {
		name           string
		disableIPMode  bool
		podName        string
		rfName         string
		rfNamespace    string
		expectedResult string
	}{
		{
			name:           "IP mode enabled (default) - returns PodIP",
			disableIPMode:  false,
			podName:        "rfr-redisfailover-0",
			rfName:         "redisfailover",
			rfNamespace:    "basic",
			expectedResult: "10.0.0.1",
		},
		{
			name:           "IP mode disabled - returns DNS name",
			disableIPMode:  true,
			podName:        "rfr-redisfailover-0",
			rfName:         "redisfailover",
			rfNamespace:    "basic",
			expectedResult: "rfr-redisfailover-0.rfr-redisfailover.basic.svc.cluster.local",
		},
		{
			name:           "IP mode disabled - pod with ordinal 1",
			disableIPMode:  true,
			podName:        "rfr-redisfailover-1",
			rfName:         "redisfailover",
			rfNamespace:    "testns",
			expectedResult: "rfr-redisfailover-1.rfr-redisfailover.testns.svc.cluster.local",
		},
		{
			name:           "IP mode disabled - invalid pod name (no ordinal)",
			disableIPMode:  true,
			podName:        "invalid-pod-name",
			rfName:         "redisfailover",
			rfNamespace:    "basic",
			expectedResult: "10.0.0.1",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert := assert.New(t)

			rf := generateRF()
			rf.Name = test.rfName
			rf.Namespace = test.rfNamespace
			rf.Spec.Redis.DisableIPMode = test.disableIPMode

			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: test.podName,
				},
				Status: corev1.PodStatus{
					PodIP: "10.0.0.1",
				},
			}

			result := rfservice.GetPodDNSName(pod, rf)
			assert.Equal(test.expectedResult, result)
		})
	}
}

func TestGetPodAddress(t *testing.T) {
	tests := []struct {
		name           string
		disableIPMode  bool
		podReady       bool
		podIP          string
		podName        string
		expectedResult string
	}{
		{
			name:           "IP mode enabled (default) - returns PodIP",
			disableIPMode:  false,
			podReady:       true,
			podIP:          "10.0.0.1",
			podName:        "rfr-redisfailover-0",
			expectedResult: "10.0.0.1",
		},
		{
			name:           "IP mode disabled, pod ready - returns DNS name",
			disableIPMode:  true,
			podReady:       true,
			podIP:          "10.0.0.1",
			podName:        "rfr-test-0",
			expectedResult: "rfr-test-0.rfr-test.testns.svc.cluster.local",
		},
		{
			name:           "IP mode disabled, pod not ready - returns PodIP",
			disableIPMode:  true,
			podReady:       false,
			podIP:          "10.0.0.1",
			podName:        "rfr-test-0",
			expectedResult: "10.0.0.1",
		},
		{
			name:           "IP mode disabled, no PodIP - returns empty",
			disableIPMode:  true,
			podReady:       true,
			podIP:          "",
			podName:        "rfr-test-0",
			expectedResult: "",
		},
		{
			name:           "IP mode disabled, pod ready, no PodIP - returns empty",
			disableIPMode:  true,
			podReady:       true,
			podIP:          "",
			podName:        "rfr-test-0",
			expectedResult: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert := assert.New(t)

			rf := generateRF()
			rf.Spec.Redis.DisableIPMode = test.disableIPMode

			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: test.podName,
				},
				Status: corev1.PodStatus{
					PodIP: test.podIP,
					Conditions: []corev1.PodCondition{
						{
							Type: corev1.PodReady,
							Status: func() corev1.ConditionStatus {
								if test.podReady {
									return corev1.ConditionTrue
								}
								return corev1.ConditionFalse
							}(),
						},
					},
				},
			}

			result := rfservice.GetPodAddress(pod, rf)
			assert.Equal(test.expectedResult, result)
		})
	}
}

func TestGetPodIPFromAddress(t *testing.T) {
	tests := []struct {
		name           string
		disableIPMode  bool
		address        string
		pods           *corev1.PodList
		expectedResult string
	}{
		{
			name:          "IP address input - returns as-is",
			disableIPMode: false,
			address:       "10.0.0.1",
			pods: &corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "rfr-redisfailover-0"},
						Status:     corev1.PodStatus{PodIP: "10.0.0.1"},
					},
				},
			},
			expectedResult: "10.0.0.1",
		},
		{
			name:          "DNS name input, IP mode disabled - resolves to PodIP",
			disableIPMode: true,
			address:       "rfr-redisfailover-0.rfr-redisfailover.testns.svc.cluster.local",
			pods: &corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "rfr-redisfailover-0"},
						Status: corev1.PodStatus{
							PodIP: "10.0.0.1",
							Conditions: []corev1.PodCondition{
								{
									Type:   corev1.PodReady,
									Status: corev1.ConditionTrue,
								},
							},
						},
					},
				},
			},
			expectedResult: "10.0.0.1",
		},
		{
			name:          "DNS name input, pod not ready - resolves by ordinal",
			disableIPMode: true,
			address:       "rfr-redisfailover-1.rfr-redisfailover.testns.svc.cluster.local",
			pods: &corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "rfr-redisfailover-1"},
						Status: corev1.PodStatus{
							PodIP: "10.0.0.2",
							Conditions: []corev1.PodCondition{
								{
									Type:   corev1.PodReady,
									Status: corev1.ConditionFalse,
								},
							},
						},
					},
				},
			},
			expectedResult: "10.0.0.2",
		},
		{
			name:          "Unknown DNS name - returns address as-is",
			disableIPMode: true,
			address:       "unknown-pod.rfr-redisfailover.testns.svc.cluster.local",
			pods: &corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "rfr-redisfailover-0"},
						Status:     corev1.PodStatus{PodIP: "10.0.0.1"},
					},
				},
			},
			expectedResult: "unknown-pod.rfr-redisfailover.testns.svc.cluster.local",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert := assert.New(t)

			rf := generateRF()
			rf.Spec.Redis.DisableIPMode = test.disableIPMode

			result := rfservice.GetPodIPFromAddress(test.address, rf, test.pods)
			assert.Equal(test.expectedResult, result)
		})
	}
}

func TestRedisServiceWithDisableIPMode(t *testing.T) {
	tests := []struct {
		name              string
		disableIPMode     bool
		exporterEnabled   bool
		expectedClusterIP string
		expectedPorts     int
		shouldCreate      bool
	}{
		{
			name:              "IP mode disabled, exporter disabled - headless service with Redis port",
			disableIPMode:     true,
			exporterEnabled:   false,
			expectedClusterIP: "None",
			expectedPorts:     1,
			shouldCreate:      true,
		},
		{
			name:              "IP mode disabled, exporter enabled - headless service with Redis and exporter ports",
			disableIPMode:     true,
			exporterEnabled:   true,
			expectedClusterIP: "None",
			expectedPorts:     2,
			shouldCreate:      true,
		},
		{
			name:              "IP mode enabled, exporter enabled - headless service with exporter port only",
			disableIPMode:     false,
			exporterEnabled:   true,
			expectedClusterIP: "None",
			expectedPorts:     1,
			shouldCreate:      true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert := assert.New(t)

			rf := generateRF()
			rf.Spec.Redis.DisableIPMode = test.disableIPMode
			rf.Spec.Redis.Exporter.Enabled = test.exporterEnabled
			rf.Spec.Redis.Port = 6379

			generatedService := corev1.Service{}

			ms := &mK8SService.Services{}
			if test.shouldCreate {
				ms.On("CreateOrUpdateService", rf.Namespace, mock.Anything).Once().Run(func(args mock.Arguments) {
					s := args.Get(1).(*corev1.Service)
					generatedService = *s
				}).Return(nil)
			}

			client := rfservice.NewRedisFailoverKubeClient(ms, log.Dummy, metrics.Dummy)
			err := client.EnsureRedisService(rf, nil, []metav1.OwnerReference{{Name: "testing"}})

			if test.shouldCreate {
				assert.NoError(err)
				assert.Equal(corev1.ClusterIPNone, generatedService.Spec.ClusterIP)
				assert.Equal(test.expectedPorts, len(generatedService.Spec.Ports))
			}
		})
	}
}

func TestGetMasterIPWithDisableIPMode(t *testing.T) {
	tests := []struct {
		name          string
		disableIPMode bool
		podReady      bool
		expectedDNS   bool
	}{
		{
			name:          "IP mode enabled - returns IP",
			disableIPMode: false,
			podReady:      true,
			expectedDNS:   false,
		},
		{
			name:          "IP mode disabled, pod ready - returns DNS name",
			disableIPMode: true,
			podReady:      true,
			expectedDNS:   true,
		},
		{
			name:          "IP mode disabled, pod not ready - returns IP",
			disableIPMode: true,
			podReady:      false,
			expectedDNS:   false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert := assert.New(t)

			rf := generateRF()
			rf.Spec.Redis.DisableIPMode = test.disableIPMode

			pods := &corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "rfr-redisfailover-0",
						},
						Status: corev1.PodStatus{
							PodIP: "10.0.0.1",
							Phase: corev1.PodRunning,
							Conditions: []corev1.PodCondition{
								{
									Type: corev1.PodReady,
									Status: func() corev1.ConditionStatus {
										if test.podReady {
											return corev1.ConditionTrue
										}
										return corev1.ConditionFalse
									}(),
								},
							},
						},
					},
				},
			}

			ms := &mK8SService.Services{}
			ms.On("GetStatefulSetPods", namespace, rfservice.GetRedisName(rf)).Once().Return(pods, nil)
			mr := &mRedisService.Client{}
			address := "10.0.0.1"
			if test.expectedDNS {
				address = "rfr-test-0.rfr-test.testns.svc.cluster.local"
			}
			mr.On("IsMaster", address, "0", "").Once().Return(true, nil)

			checker := rfservice.NewRedisFailoverChecker(ms, mr, log.DummyLogger{}, metrics.Dummy)
			master, err := checker.GetMasterIP(rf)

			assert.NoError(err)
			if test.expectedDNS {
				assert.Contains(master, ".svc.cluster.local")
				assert.Contains(master, "rfr-test")
			} else {
				assert.Equal("10.0.0.1", master)
			}
		})
	}
}

func TestCheckAllSlavesFromMasterWithDisableIPMode(t *testing.T) {
	tests := []struct {
		name          string
		disableIPMode bool
		master        string
		slave         string
		shouldError   bool
		errorContains string
	}{
		{
			name:          "IP mode enabled - master and slave both IPs",
			disableIPMode: false,
			master:        "10.0.0.1",
			slave:         "10.0.0.1", // slave pod (10.0.0.2) reports master as 10.0.0.1
			shouldError:   false,
		},
		{
			name:          "IP mode disabled - master DNS, slave DNS",
			disableIPMode: true,
			master:        "rfr-test-0.rfr-test.testns.svc.cluster.local",
			slave:         "rfr-test-0.rfr-test.testns.svc.cluster.local", // Slave reports DNS name
			shouldError:   false,
		},
		{
			name:          "IP mode disabled - master DNS, slave IP (should error for reconfiguration)",
			disableIPMode: true,
			master:        "rfr-test-0.rfr-test.testns.svc.cluster.local",
			slave:         "10.0.0.1", // Slave reports IP even though master is DNS
			shouldError:   true,       // Should error to force reconfiguration to use DNS
			errorContains: "should use DNS name",
		},
		{
			name:          "IP mode disabled - master DNS, slave different IP",
			disableIPMode: true,
			master:        "rfr-test-0.rfr-test.testns.svc.cluster.local",
			slave:         "10.0.0.2",
			shouldError:   true,
			errorContains: "don't have the master",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert := assert.New(t)

			rf := generateRF()
			rf.Spec.Redis.DisableIPMode = test.disableIPMode

			// Create pods list - master pod and slave pod
			// Master is always pod 0, slave is always pod 1
			pods := &corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "rfr-test-0",
						},
						Status: corev1.PodStatus{
							PodIP: "10.0.0.1",
							Phase: corev1.PodRunning,
							Conditions: []corev1.PodCondition{
								{
									Type:   corev1.PodReady,
									Status: corev1.ConditionTrue,
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "rfr-test-1",
						},
						Status: corev1.PodStatus{
							PodIP: "10.0.0.2",
							Phase: corev1.PodRunning,
							Conditions: []corev1.PodCondition{
								{
									Type:   corev1.PodReady,
									Status: corev1.ConditionTrue,
								},
							},
						},
					},
				},
			}

			ms := &mK8SService.Services{}
			ms.On("GetStatefulSetPods", namespace, rfservice.GetRedisName(rf)).Once().Return(pods, nil)
			ms.On("UpdatePodLabels", namespace, mock.AnythingOfType("string"), mock.Anything).Return(nil)
			ms.On("UpdatePodAnnotations", namespace, mock.AnythingOfType("string"), mock.Anything).Return(nil)
			ms.On("RemovePodAnnotation", namespace, mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)

			mr := &mRedisService.Client{}
			// The function iterates through ALL pods and calls GetSlaveOf for each
			// The check `if podAddress == master` only affects label/annotation setting
			// GetSlaveOf is always called for every pod

			// Determine addresses for pods based on disableIPMode
			pod0Address := "10.0.0.1"
			pod1Address := "10.0.0.2"
			if test.disableIPMode {
				pod0Address = "rfr-test-0.rfr-test.testns.svc.cluster.local"
				pod1Address = "rfr-test-1.rfr-test.testns.svc.cluster.local"
			}

			// GetSlaveOf is called for all pods
			// Pod 0 (master) will return empty string or its own address
			// Pod 1 (slave) will return the master address
			mr.On("GetSlaveOf", pod0Address, "0", "").Once().Return("", nil)         // Master has no slaveof
			mr.On("GetSlaveOf", pod1Address, "0", "").Once().Return(test.slave, nil) // Slave points to master

			checker := rfservice.NewRedisFailoverChecker(ms, mr, log.DummyLogger{}, metrics.Dummy)
			err := checker.CheckAllSlavesFromMaster(test.master, rf)

			if test.shouldError {
				assert.Error(err)
				if test.errorContains != "" {
					assert.Contains(err.Error(), test.errorContains)
				}
			} else {
				assert.NoError(err)
			}
		})
	}
}

func TestSetRedisCustomConfigWithDisableIPMode(t *testing.T) {
	tests := []struct {
		name                   string
		disableIPMode          bool
		podReady               bool
		address                string
		expectedConfigContains string
	}{
		{
			name:                   "IP mode enabled - no replica-announce-ip added",
			disableIPMode:          false,
			podReady:               true,
			address:                "10.0.0.1",
			expectedConfigContains: "",
		},
		{
			name:                   "IP mode disabled, pod ready - replica-announce-ip added",
			disableIPMode:          true,
			podReady:               true,
			address:                "10.0.0.1",
			expectedConfigContains: "replica-announce-ip rfr-test-0.rfr-test.testns.svc.cluster.local",
		},
		{
			name:                   "IP mode disabled, pod not ready - no replica-announce-ip added",
			disableIPMode:          true,
			podReady:               false,
			address:                "10.0.0.1",
			expectedConfigContains: "",
		},
		{
			name:                   "IP mode disabled, address is DNS name, pod ready - replica-announce-ip added",
			disableIPMode:          true,
			podReady:               true,
			address:                "rfr-test-0.rfr-test.testns.svc.cluster.local",
			expectedConfigContains: "replica-announce-ip rfr-test-0.rfr-test.testns.svc.cluster.local",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert := assert.New(t)

			rf := generateRF()
			rf.Spec.Redis.DisableIPMode = test.disableIPMode
			rf.Spec.Redis.CustomConfig = []string{"some-config"}

			pods := &corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "rfr-test-0",
						},
						Status: corev1.PodStatus{
							PodIP: "10.0.0.1",
							Conditions: []corev1.PodCondition{
								{
									Type: corev1.PodReady,
									Status: func() corev1.ConditionStatus {
										if test.podReady {
											return corev1.ConditionTrue
										}
										return corev1.ConditionFalse
									}(),
								},
							},
						},
					},
				},
			}

			ms := &mK8SService.Services{}
			// getRedisPodMemoryUsage always calls GetStatefulSetPods
			ms.On("GetStatefulSetPods", namespace, rfservice.GetRedisName(rf)).Once().Return(pods, nil)
			// If disableIPMode is true, SetRedisCustomConfig also calls GetStatefulSetPods
			if test.disableIPMode {
				ms.On("GetStatefulSetPods", namespace, rfservice.GetRedisName(rf)).Once().Return(pods, nil)
			}
			ms.On("GetPod", namespace, mock.AnythingOfType("string")).Return(nil, nil).Maybe()

			mr := &mRedisService.Client{}
			var capturedConfig []string
			mr.On("SetCustomRedisConfig", test.address, "0", mock.MatchedBy(func(config []string) bool {
				capturedConfig = config
				return true
			}), "").Once().Return(nil)

			healer := rfservice.NewRedisFailoverHealer(ms, mr, log.DummyLogger{})
			err := healer.SetRedisCustomConfig(test.address, rf)

			assert.NoError(err)
			if test.expectedConfigContains != "" {
				found := false
				for _, config := range capturedConfig {
					if config == test.expectedConfigContains {
						found = true
						break
					}
				}
				assert.True(found, "Expected config '%s' not found in %v", test.expectedConfigContains, capturedConfig)
			} else {
				// Verify replica-announce-ip is NOT in the config
				for _, config := range capturedConfig {
					assert.NotContains(config, "replica-announce-ip", "replica-announce-ip should not be present when IP mode is enabled or pod is not ready")
				}
			}
		})
	}
}
