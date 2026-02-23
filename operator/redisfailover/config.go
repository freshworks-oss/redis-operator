package redisfailover

// Config is the configuration for the redis operator.
type Config struct {
	ListenAddress            string
	MetricsPath              string
	Concurrency              int
	SupportedNamespacesRegex string
	// OperatorShardID is the shard ID for label-based sharding. Only RF CRs with
	// Only RF CRs with label redis-failover.freshworks.com/shard=<OperatorShardID> are reconciled by this instance.
	OperatorShardID string
}
