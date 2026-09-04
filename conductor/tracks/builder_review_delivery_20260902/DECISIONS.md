---
type: decision-log
title: Builder Review Links and Live Delivery QA Decisions
---

# Decisions

## Review URL is a selector, not a capability

The dynamic review URL contains only the opaque Conductor track ID in the
account-document fragment. Existing authentication, workspace authorization,
active manifest verification, exact file digest checks, and expiry rules remain
the only route to rendering. The fragment is not sent in HTTP requests and must
never be exchanged for membership or publication authority.

## Public delivery stays on the verified-domain lifecycle

A protected member-review link and an anonymous public site solve different
problems. Anonymous or customer-facing delivery continues to require an exact
HTTPS origin, DNS proof, Architect authority, active signed authorization, and
explicit publication activation. This track does not disguise a review link as
public hosting.

## Live QA may report an external block

The promotion run may exercise DNS publication only with a genuinely controlled
hostname. Lack of that hostname is recorded as an external blocker, not worked
around with fabricated resolver results, altered production data, or a false
acceptance claim.
