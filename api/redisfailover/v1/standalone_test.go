package v1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func generateStandaloneRedisFailover(name string, standalone bool) *RedisFailover {
	return &RedisFailover{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "namespace",
		},
		Spec: RedisFailoverSpec{
			Standalone: standalone,
		},
	}
}

func TestStandalone(t *testing.T) {
	tests := []struct {
		name        string
		expectation bool
		standalone  bool
	}{
		{
			name:        "without standalone",
			expectation: false,
			standalone:  false,
		},
		{
			name:        "with standalone",
			expectation: true,
			standalone:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rf := generateStandaloneRedisFailover("test", test.standalone)
			assert.Equal(t, test.expectation, rf.Standalone())
		})
	}
}

func TestSentinelsAllowedWithStandalone(t *testing.T) {
	rf := generateStandaloneRedisFailover("test", true)
	assert.False(t, rf.SentinelsAllowed())
}
