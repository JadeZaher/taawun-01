# Architecture Decisions: Taqwa Ethics Audit & Anti-Gharar Engine

**Track ID**: `taqwa_ethics_guardrails_20260810`

---

### Decision 1: Constructive Guidance over Generic Rejection
- **Ruling**: When a prompt fails the Taqwa audit due to Riba interest calculations, the system returns specific constructive Islamic finance suggestions (e.g. Murabaha profit-margin or Mudarabah equity sharing).
- **Rationale**: Educates creators and encourages ethical alternatives rather than frustrating the user.
