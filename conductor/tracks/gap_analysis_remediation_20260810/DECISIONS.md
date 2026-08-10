# Architecture Decisions: Platform Gap Analysis Remediation

**Track ID**: `gap_analysis_remediation_20260810`

---

### Decision 1: Embedded DuckDB for Analytics
- **Ruling**: Store raw transactional data in SQLite, and aggregate analytics via DuckDB in-memory queries.
- **Rationale**: Eliminates the overhead of running a large PostgreSQL / ClickHouse cluster while providing sub-second analytical aggregations across community workspaces.
