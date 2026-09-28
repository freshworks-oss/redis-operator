package v1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name                   string
		rfName                 string
		rfEngine               DatabaseEngine
		rfBootstrapNode        *BootstrapSettings
		rfRedisCustomConfig    []string
		rfSentinelCustomConfig []string
		rfStandalone           bool
		rfRedisReplicas        int32
		expectedError          string
		expectedBootstrapNode  *BootstrapSettings
	}{
		{
			name:   "populates default values",
			rfName: "test",
		},
		{
			name:          "errors on too long of name",
			rfName:        "some-super-absurdely-unnecessarily-long-name-that-will-most-definitely-fail",
			expectedError: "name length can't be higher than 48",
		},
		{
			name:          "errors on invalid engine",
			rfName:        "test",
			rfEngine:      DatabaseEngine("Other"),
			expectedError: "invalid engine",
		},
		{
			name:     "Valkey engine uses default Valkey image",
			rfName:   "test",
			rfEngine: ValkeyEngine,
		},
		{
			name:                   "SentinelCustomConfig provided",
			rfName:                 "test",
			rfSentinelCustomConfig: []string{"failover-timeout 500"},
		},
		{
			name:            "BootstrapNode provided without a host",
			rfName:          "test",
			rfBootstrapNode: &BootstrapSettings{},
			expectedError:   "BootstrapNode must include a host when provided",
		},
		{
			name:   "SentinelCustomConfig not provided",
			rfName: "test",
		},
		{
			name:                  "Populates default bootstrap port when valid",
			rfName:                "test",
			rfBootstrapNode:       &BootstrapSettings{Host: "127.0.0.1"},
			expectedBootstrapNode: &BootstrapSettings{Host: "127.0.0.1", Port: "6379"},
		},
		{
			name:                  "Allows for specifying boostrap port",
			rfName:                "test",
			rfBootstrapNode:       &BootstrapSettings{Host: "127.0.0.1", Port: "6380"},
			expectedBootstrapNode: &BootstrapSettings{Host: "127.0.0.1", Port: "6380"},
		},
		{
			name:                "Appends applied custom config to default initial values",
			rfName:              "test",
			rfRedisCustomConfig: []string{"tcp-keepalive 60"},
		},
		{
			name:                  "Appends applied custom config to default initial values when bootstrapping",
			rfName:                "test",
			rfRedisCustomConfig:   []string{"tcp-keepalive 60"},
			rfBootstrapNode:       &BootstrapSettings{Host: "127.0.0.1"},
			expectedBootstrapNode: &BootstrapSettings{Host: "127.0.0.1", Port: "6379"},
		},
		{
			name:         "Standalone populates defaults",
			rfName:       "test",
			rfStandalone: true,
		},
		{
			name:                  "Standalone allows BootstrapNode, to seed from an external host",
			rfName:                "test",
			rfStandalone:          true,
			rfBootstrapNode:       &BootstrapSettings{Host: "127.0.0.1"},
			expectedBootstrapNode: &BootstrapSettings{Host: "127.0.0.1", Port: "6379"},
		},
		{
			name:            "Standalone rejects an explicit conflicting replica count",
			rfName:          "test",
			rfStandalone:    true,
			rfRedisReplicas: 3,
			expectedError:   "standalone mode requires redis.replicas to be 1",
		},
		{
			name:                  "Standalone bootstrapping with allowSentinels populates sentinel defaults",
			rfName:                "test",
			rfStandalone:          true,
			rfBootstrapNode:       &BootstrapSettings{Host: "127.0.0.1", AllowSentinels: true},
			expectedBootstrapNode: &BootstrapSettings{Host: "127.0.0.1", Port: "6379", AllowSentinels: true},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert := assert.New(t)
			rf := generateRedisFailover(test.rfName, test.rfBootstrapNode)
			rf.Spec.Engine = test.rfEngine
			rf.Spec.Redis.CustomConfig = test.rfRedisCustomConfig
			rf.Spec.Sentinel.CustomConfig = test.rfSentinelCustomConfig
			rf.Spec.Standalone = test.rfStandalone
			rf.Spec.Redis.Replicas = test.rfRedisReplicas

			err := rf.Validate()

			if test.expectedError == "" {
				assert.NoError(err)

				defaultImg := defaultImage
				if test.rfEngine == ValkeyEngine {
					defaultImg = defaultValkeyImage
				}

				expectedRedisCustomConfig := []string{
					"replica-priority 100",
				}

				if test.rfBootstrapNode != nil {
					expectedRedisCustomConfig = []string{
						"replica-priority 0",
					}
				}

				expectedRedisCustomConfig = append(expectedRedisCustomConfig, test.rfRedisCustomConfig...)
				expectedSentinelCustomConfig := defaultSentinelCustomConfig
				if len(test.rfSentinelCustomConfig) > 0 {
					expectedSentinelCustomConfig = test.rfSentinelCustomConfig
				}

				expectedRedisReplicas := int32(defaultRedisNumber)
				expectedSentinelReplicas := int32(defaultSentinelNumber)
				if test.rfStandalone {
					expectedRedisReplicas = 1
					expectedSentinelReplicas = 0
					// A standalone pod still bootstrapping from an external host with
					// allowSentinels is the one exception: same as SentinelsAllowed().
					if test.rfBootstrapNode != nil && test.rfBootstrapNode.AllowSentinels {
						expectedSentinelReplicas = int32(defaultSentinelNumber)
					}
				}

				expectedRF := &RedisFailover{
					ObjectMeta: metav1.ObjectMeta{
						Name:      test.rfName,
						Namespace: "namespace",
					},
					Spec: RedisFailoverSpec{
						Engine:     test.rfEngine,
						Standalone: test.rfStandalone,
						Redis: RedisSettings{
							Image:                    defaultImg,
							Replicas:                 expectedRedisReplicas,
							Port:                     defaultRedisPort,
							ReservedPodMemoryPercent: defaultReservedPodMemoryPercent,
							Exporter: Exporter{
								Image: defaultExporterImage,
							},
							CustomConfig: expectedRedisCustomConfig,
						},
						Sentinel: SentinelSettings{
							Image:        defaultImg,
							Replicas:     expectedSentinelReplicas,
							CustomConfig: expectedSentinelCustomConfig,
							Exporter: Exporter{
								Image: defaultSentinelExporterImage,
							},
						},
						BootstrapNode: test.expectedBootstrapNode,
					},
				}
				assert.Equal(expectedRF, rf)
			} else {
				if assert.Error(err) {
					assert.Contains(err.Error(), test.expectedError)
				}
			}
		})
	}
}
