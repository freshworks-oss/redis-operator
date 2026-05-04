package service

import (
	redisfailoverv1 "github.com/freshworks/redis-operator/api/redisfailover/v1"
)

// DatabaseEngineProvider resolves Redis vs Valkey container binaries and CLI auth for shell snippets.
type DatabaseEngineProvider interface {
	ServerBinary() string
	CLIBinary() string
	// CLIAuthEnvName is exported before invoking the CLI when REDIS_PASSWORD is set (e.g. REDISCLI_AUTH, VALKEYCLI_AUTH).
	CLIAuthEnvName() string
}

type redisEngine struct{}

func (redisEngine) ServerBinary() string    { return "redis-server" }
func (redisEngine) CLIBinary() string         { return "redis-cli" }
func (redisEngine) CLIAuthEnvName() string   { return "REDISCLI_AUTH" }

type valkeyEngine struct{}

func (valkeyEngine) ServerBinary() string    { return "valkey-server" }
func (valkeyEngine) CLIBinary() string       { return "valkey-cli" }
func (valkeyEngine) CLIAuthEnvName() string { return "VALKEYCLI_AUTH" }

// EngineFor returns the engine implementation for pod generation. Empty or Redis uses Redis binaries; Valkey uses Valkey binaries.
func EngineFor(rf *redisfailoverv1.RedisFailover) DatabaseEngineProvider {
	switch rf.Spec.DatabaseEngine {
	case redisfailoverv1.DatabaseEngineValkey:
		return valkeyEngine{}
	default:
		return redisEngine{}
	}
}
