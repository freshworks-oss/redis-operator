package v1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDatabaseEngineImageMismatchHints(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		rf   *RedisFailover
		want int // number of hints expected
	}{
		{
			name: "nil",
			rf:   nil,
			want: 0,
		},
		{
			name: "defaults only no explicit images",
			rf: &RedisFailover{
				Spec: RedisFailoverSpec{
					DatabaseEngine: DatabaseEngineValkey,
				},
			},
			want: 0,
		},
		{
			name: "Valkey engine redis images",
			rf: &RedisFailover{
				Spec: RedisFailoverSpec{
					DatabaseEngine: DatabaseEngineValkey,
					Redis: RedisSettings{
						Image: "redis:7-alpine",
					},
					Sentinel: SentinelSettings{
						Image: "redis:7-alpine",
					},
				},
			},
			want: 2,
		},
		{
			name: "Valkey engine matching images",
			rf: &RedisFailover{
				Spec: RedisFailoverSpec{
					DatabaseEngine: DatabaseEngineValkey,
					Redis: RedisSettings{
						Image: "valkey/valkey:8",
					},
					Sentinel: SentinelSettings{
						Image: "valkey/valkey:8",
					},
				},
			},
			want: 0,
		},
		{
			name: "Redis engine valkey images",
			rf: &RedisFailover{
				Spec: RedisFailoverSpec{
					DatabaseEngine: DatabaseEngineRedis,
					Redis: RedisSettings{
						Image: "valkey/valkey:8",
					},
					Sentinel: SentinelSettings{
						Image: "valkey/valkey:8",
					},
				},
			},
			want: 2,
		},
		{
			name: "unset engine valkey redis image only",
			rf: &RedisFailover{
				Spec: RedisFailoverSpec{
					Redis: RedisSettings{
						Image: "valkey/valkey:8",
					},
				},
			},
			want: 1,
		},
		{
			name: "Redis engine custom image without redis or valkey in name",
			rf: &RedisFailover{
				Spec: RedisFailoverSpec{
					DatabaseEngine: DatabaseEngineRedis,
					Redis: RedisSettings{
						Image: "myregistry.io/team/cache-runner:v2",
					},
					Sentinel: SentinelSettings{
						Image: "myregistry.io/team/cache-runner:v2",
					},
				},
			},
			want: 2,
		},
		{
			name: "Redis engine explicit redis image no hints",
			rf: &RedisFailover{
				Spec: RedisFailoverSpec{
					DatabaseEngine: DatabaseEngineRedis,
					Redis: RedisSettings{
						Image: "redis:7-alpine",
					},
					Sentinel: SentinelSettings{
						Image: "redis:7-alpine",
					},
				},
			},
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := DatabaseEngineImageMismatchHints(tt.rf)
			assert.Len(t, got, tt.want)
		})
	}
}
