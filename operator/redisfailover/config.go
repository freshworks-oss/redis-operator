package redisfailover

// Config is the configuration for the redis operator.
type Config struct {
	ListenAddress            string
	MetricsPath              string
	Concurrency              int
	SupportedNamespacesRegex string
	// OperatorGroupID is the group ID for label-based grouping.
	// Only RF CRs with label redis-failover.freshworks.com/operator-group=<OperatorGroupID>
	// are reconciled by this instance.
	OperatorGroupID string
}
