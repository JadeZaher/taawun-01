# Financial primitives

`azoa.go` is a durable orchestration boundary, not a money ledger. Generated and
browser applications may request vetted sandbox quests, but they never custody
funds, persist balances, assert settlement, or call a provider directly.

The SQLite sandbox stores orchestration quests, provider intents, approvals,
and a hash-linked append-only event trail. Monetary values are integer minor
units with an explicit currency. Every mutation uses an idempotency or expected
aggregate version, and an intent can become settled only from `EXECUTING` with
a non-empty reconciliation reference.

Provider implementations own communication with a real AZOA node. Credentials,
network retries, reconciliation, and external idempotency belong in that
adapter; this package supplies no live provider or invented endpoint. The
built-in sandbox provider accepts an intent into pending execution and requires
an explicit manual reconciliation outcome. `Start` must be idempotent for the
provided key because an unconfirmed provider call remains `EXECUTING` and can
be retried after a timeout or process restart.

The vetted flow catalog is the policy boundary for donation, marketplace
escrow, revenue split, Zakat, Qard Hasan, volunteer stipend, and multi-party
approval. Extend the catalog deliberately rather than accepting arbitrary
financial action names or executable flow definitions. Only declared quest
parties may approve, and revenue allocations must cover exactly those parties.

`federation.go` owns cross-node signatures. Financial orchestration may place a
validated quest projection inside its existing envelope, but must not create a
second signature scheme. A valid envelope proves only message provenance; it
does not prove settlement. A live federation deployment still needs a durable
inbox, mutually trusted node-key registry, transport authentication, and a real
provider contract.
