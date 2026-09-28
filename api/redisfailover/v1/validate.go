package v1

import (
	"errors"
	"fmt"
	"strconv"
)

const (
	maxNameLength = 48
)

// Validate set the values by default if not defined and checks if the values given are valid
func (r *RedisFailover) Validate() error {
	if len(r.Name) > maxNameLength {
		return fmt.Errorf("name length can't be higher than %d", maxNameLength)
	}

	switch r.Spec.Engine {
	case "", RedisEngine, ValkeyEngine:
	default:
		return fmt.Errorf("invalid engine %q, must be Redis, Valkey, or omitted", r.Spec.Engine)
	}

	defaultImageForEngine := defaultImage
	if r.Spec.Engine == ValkeyEngine {
		defaultImageForEngine = defaultValkeyImage
	}

	if r.Bootstrapping() {
		if r.Spec.BootstrapNode.Host == "" {
			return errors.New("BootstrapNode must include a host when provided")
		}

		if r.Spec.BootstrapNode.Port == "" {
			r.Spec.BootstrapNode.Port = strconv.Itoa(defaultRedisPort)
		}
		r.Spec.Redis.CustomConfig = deduplicateStr(append(bootstrappingRedisCustomConfig, r.Spec.Redis.CustomConfig...))
	} else {
		r.Spec.Redis.CustomConfig = deduplicateStr(append(defaultRedisCustomConfig, r.Spec.Redis.CustomConfig...))
	}

	if r.Spec.Standalone {
		// bootstrapNode is allowed alongside standalone: the single pod replicates
		// from bootstrapNode.Host until the user removes bootstrapNode from the spec,
		// at which point checkAndHealStandaloneMode promotes it via the existing
		// zero-masters/SetOldestAsMaster path. See CheckAndHeal in checker.go.
		if r.Spec.Redis.Replicas != 0 && r.Spec.Redis.Replicas != 1 {
			return errors.New("standalone mode requires redis.replicas to be 1 (or omitted)")
		}
		r.Spec.Redis.Replicas = 1
	}

	if r.Spec.Redis.Image == "" {
		r.Spec.Redis.Image = defaultImageForEngine
	}

	if r.Spec.Sentinel.Image == "" {
		r.Spec.Sentinel.Image = defaultImageForEngine
	}

	if r.Spec.Redis.Replicas <= 0 {
		r.Spec.Redis.Replicas = defaultRedisNumber
	}

	if r.Spec.Redis.Port <= 0 {
		r.Spec.Redis.Port = defaultRedisPort
	}

	if r.Spec.Redis.ReservedPodMemoryPercent <= 0 {
		r.Spec.Redis.ReservedPodMemoryPercent = defaultReservedPodMemoryPercent
	}

	// Standalone normally has no Sentinel at all, so this stays 0 unless a
	// standalone pod is still bootstrapping from an external host with
	// allowSentinels — same condition as SentinelsAllowed().
	if r.Spec.Sentinel.Replicas <= 0 && (!r.Spec.Standalone || r.SentinelsAllowed()) {
		r.Spec.Sentinel.Replicas = defaultSentinelNumber
	}

	if r.Spec.Redis.Exporter.Image == "" {
		r.Spec.Redis.Exporter.Image = defaultExporterImage
	}

	if r.Spec.Sentinel.Exporter.Image == "" {
		r.Spec.Sentinel.Exporter.Image = defaultSentinelExporterImage
	}

	if len(r.Spec.Sentinel.CustomConfig) == 0 {
		r.Spec.Sentinel.CustomConfig = defaultSentinelCustomConfig
	}

	return nil
}

func deduplicateStr(strSlice []string) []string {
	allKeys := make(map[string]bool)
	list := []string{}
	for _, item := range strSlice {
		if _, value := allKeys[item]; !value {
			allKeys[item] = true
			list = append(list, item)
		}
	}
	return list
}
