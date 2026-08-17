# SQLite runtime contract

The canonical service uses one SQLite file for identity and the small durable
control-plane stores owned by each package. Every package owns idempotent schema
creation for its own tables; `database.go` owns only the shared connection and
legacy identity/workspace tables.

The connection URI enables foreign keys on every pooled connection, WAL for
concurrent readers, a bounded busy timeout, `NORMAL` synchronous durability, and
immediate write transactions. Keep monetary values in integer minor units and
perform lifecycle transitions inside explicit transactions with optimistic
versions. Schema changes must be additive and safe to run repeatedly.
