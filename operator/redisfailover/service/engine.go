package service

import (
	redisfailoverv1 "github.com/freshworks/redis-operator/api/redisfailover/v1"
)

// DatabaseEngineProvider resolves Redis vs Valkey container binaries for shell snippets.
type DatabaseEngineProvider interface {
	ServerBinary() string
	CLIBinary() string
}

type redisEngine struct{}

func (redisEngine) ServerBinary() string { return "redis-server" }
func (redisEngine) CLIBinary() string    { return "redis-cli" }

type valkeyEngine struct{}

func (valkeyEngine) ServerBinary() string { return "valkey-server" }
func (valkeyEngine) CLIBinary() string    { return "valkey-cli" }

// EngineFor returns the engine implementation for pod generation. Empty or Redis uses Redis binaries; Valkey uses Valkey binaries.
func EngineFor(rf *redisfailoverv1.RedisFailover) DatabaseEngineProvider {
	switch rf.Spec.Engine {
	case redisfailoverv1.ValkeyEngine:
		return valkeyEngine{}
	default:
		return redisEngine{}
	}
}
