# SQLite runtime contract

The canonical service uses one GORM-owned SQLite connection for identity and the
small durable control-plane stores owned by each package. Every package owns
idempotent schema creation for its own tables; `database.go` owns the shared
connection and the legacy identity/workspace/notification tables. Append-only
stores may use `SQLDB` for their hand-tuned optimistic SQL and transactions, but
must not open a second connection to the application database.

The connection URI enables foreign keys on every pooled connection, WAL for
concurrent readers, a bounded busy timeout, `NORMAL` synchronous durability, and
immediate write transactions. Keep monetary values in integer minor units and
perform lifecycle transitions inside explicit transactions with optimistic
versions. Schema changes must be additive and safe to run repeatedly.

Do not call unconstrained `AutoMigrate` on the production database. SQLite may
rebuild a table to alter a column. Use explicit `Migrator` existence checks and
only create missing tables, columns, constraints, or indexes so `/data/taawun.db`
can be adopted in place without copying or rewriting customer records. GORM SQL
logging stays disabled because interpolated identity values are sensitive.

## Account tombstones

Account deletion retains the `users.id` tombstone because verified-domain,
publication, OAuth, and other audit rows intentionally reference it. The
lifecycle transaction replaces identity and credential material, marks the user
deleted, rotates the first-party session version, revokes OAuth credentials and
consents, and removes non-audit membership/notification rows. Do not restore a
hard delete or weaken those audit foreign keys.
