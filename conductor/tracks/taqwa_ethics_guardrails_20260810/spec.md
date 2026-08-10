# Spec: Taqwa Ethics Audit & Anti-Gharar Engine

**Track ID**: `taqwa_ethics_guardrails_20260810`  
**Status**: Completed  
**Authoritative Location**: `pkg/ethics/haram_check.go`, `pkg/ethics/anti_gharar.go`

---

## 1. Requirements

### Functional Requirements
- **FR-1**: `AuditPrompt(prompt)` scans for prohibited keywords (gambling, interest loans, etc.) and Riba regex patterns.
- **FR-2**: `AuditCode(code)` inspects generated source code for interest computation functions.
- **FR-3**: `ValidateForCheckout(spec)` validates staging preview status, expiration timestamp, and resource limits before cloud deployment checkout.

---

## 2. Acceptance Criteria
- [x] Compliant prompts pass audit with `Passed = true`.
- [x] Prompts requesting conventional interest calculations fail with detailed violation notices and Murabaha suggestions.
- [x] Unit tests in `pkg/ethics/ethics_test.go` pass 100%.
