package v1

import "strings"

// imageNameSuggestsRedisOrValkey is a loose substring check for typical Redis/Valkey image names.
func imageNameSuggestsRedisOrValkey(lower string) bool {
	return strings.Contains(lower, "redis") || strings.Contains(lower, "valkey")
}

// DatabaseEngineImageMismatchHints returns soft UX messages when spec.databaseEngine and
// explicit image references look inconsistent under a simple naming heuristic (substring).
// This does not reject applies; registry paths and mirror names may legitimately omit "valkey"/"redis".
func DatabaseEngineImageMismatchHints(rf *RedisFailover) []string {
	if rf == nil {
		return nil
	}
	var hints []string
	eng := rf.Spec.DatabaseEngine
	ri := strings.ToLower(rf.Spec.Redis.Image)
	si := strings.ToLower(rf.Spec.Sentinel.Image)

	if eng == DatabaseEngineValkey {
		if rf.Spec.Redis.Image != "" && !strings.Contains(ri, "valkey") {
			hints = append(hints, `spec.redis.image does not contain "valkey"; use a Valkey image or set spec.redis.command to match binaries inside the image`)
		}
		if rf.Spec.Sentinel.Image != "" && !strings.Contains(si, "valkey") {
			hints = append(hints, `spec.sentinel.image does not contain "valkey"; use a Valkey image or set spec.sentinel.command to match binaries inside the image`)
		}
	}
	if eng == "" || eng == DatabaseEngineRedis {
		if rf.Spec.Redis.Image != "" && strings.Contains(ri, "valkey") {
			hints = append(hints, `spec.redis.image appears to be Valkey but databaseEngine is Redis or unset; set databaseEngine: Valkey or use a Redis image`)
		}
		if rf.Spec.Sentinel.Image != "" && strings.Contains(si, "valkey") {
			hints = append(hints, `spec.sentinel.image appears to be Valkey but databaseEngine is Redis or unset; set databaseEngine: Valkey or use a Redis image`)
		}
		if rf.Spec.Redis.Image != "" && !imageNameSuggestsRedisOrValkey(ri) {
			hints = append(hints, `spec.redis.image does not contain "redis" or "valkey"; confirm the image matches databaseEngine (Redis protocol) and supplies redis-server/redis-cli or override command`)
		}
		if rf.Spec.Sentinel.Image != "" && !imageNameSuggestsRedisOrValkey(si) {
			hints = append(hints, `spec.sentinel.image does not contain "redis" or "valkey"; confirm the image matches databaseEngine and supplies redis-server for sentinel mode or override command`)
		}
	}
	return hints
}
