package v1

// Bootstrapping returns true when a BootstrapNode is provided to the RedisFailover spec. Otherwise, it returns false.
func (r *RedisFailover) Bootstrapping() bool {
	return r.Spec.BootstrapNode != nil
}

// Standalone returns true when the RedisFailover is configured to run a single
// self-contained Redis instance with no Sentinel deployed at all.
func (r *RedisFailover) Standalone() bool {
	return r.Spec.Standalone
}

// SentinelsAllowed returns true if not Standalone, and either not Bootstrapping or
// BootstrapNode settings allow sentinels to exist.
func (r *RedisFailover) SentinelsAllowed() bool {
	if r.Standalone() {
		return false
	}
	bootstrapping := r.Bootstrapping()
	return !bootstrapping || (bootstrapping && r.Spec.BootstrapNode.AllowSentinels)
}
