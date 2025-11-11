# Technical Debt GitHub Issues - Summary

**Created**: November 11, 2025  
**Purpose**: Track GitHub issues created for technical debt items

---

## Overview

This document summarizes the GitHub parent issues created for all technical debt items identified in `docs/technical-debt.md`. Each issue is a parent issue that can be broken down into sub-issues for implementation.

---

## Created Issues

### Priority 1: Critical (Must Fix Before Production)

#### [#57 - PostgreSQL Support](https://github.com/sofatutor/llm-proxy/issues/57) ✅ Existing
- **Status**: Already existed
- **Impact**: HIGH - SQLite has concurrency limitations
- **Effort**: 2-3 weeks
- **Description**: Enable PostgreSQL as optional database backend

#### [#109 - Database Migration System](https://github.com/sofatutor/llm-proxy/issues/109) ✨ New
- **Status**: Created
- **Impact**: HIGH - Schema changes are error-prone
- **Effort**: 1 week
- **Description**: Implement proper database migration system with tracking and rollback

#### [#110 - Distributed Rate Limiting](https://github.com/sofatutor/llm-proxy/issues/110) ✨ New
- **Status**: Created
- **Impact**: MEDIUM-HIGH - Rate limiting is per-instance
- **Effort**: 1-2 weeks
- **Description**: Implement Redis-backed distributed rate limiting across all instances

---

### Priority 2: Important (Should Fix Soon)

#### [#111 - Cache Invalidation API](https://github.com/sofatutor/llm-proxy/issues/111) ✨ New
- **Status**: Created
- **Impact**: MEDIUM - Can't purge cache on demand
- **Effort**: 3-5 days
- **Description**: Implement manual cache invalidation and purge capabilities

#### [#112 - Durable Event Queue with Guaranteed Delivery](https://github.com/sofatutor/llm-proxy/issues/112) ✨ New
- **Status**: Created
- **Impact**: MEDIUM - Events can be lost if dispatcher lags
- **Effort**: 1-2 weeks
- **Description**: Implement durable queue with at-least-once delivery semantics

#### [#113 - Unified HTTP Server (Single Port)](https://github.com/sofatutor/llm-proxy/issues/113) ✨ New
- **Status**: Created
- **Impact**: MEDIUM - Complicates deployment
- **Effort**: 1 week
- **Description**: Refactor to single HTTP server with route prefixes

#### [#114 - Built-in HTTPS Support](https://github.com/sofatutor/llm-proxy/issues/114) ✨ New
- **Status**: Created
- **Impact**: MEDIUM - Extra deployment step
- **Effort**: 1 week
- **Description**: Add built-in HTTPS with optional Let's Encrypt integration

---

### Priority 3-4: Minor/Deferred (Nice to Have)

#### [#115 - Comprehensive Package Documentation](https://github.com/sofatutor/llm-proxy/issues/115) ✨ New
- **Status**: Created
- **Impact**: LOW - Hard to understand packages quickly
- **Effort**: 8-20 hours total
- **Description**: Expand package READMEs with architecture, examples, and guidance

#### [#116 - Optimizations and Future Enhancements](https://github.com/sofatutor/llm-proxy/issues/116) ✨ New
- **Status**: Created (collection issue)
- **Impact**: LOW - Nice to have
- **Effort**: Varies
- **Description**: Collection of low-priority optimizations and future features
- **Includes**:
  - Token timestamp extraction from UUIDv7
  - Vary header parsing optimization
  - Request/response transformation pipeline
  - Multi-provider load balancing
  - Real-time metrics dashboard

---

## Issue Statistics

### By Priority
- **Priority 1 (Critical)**: 3 issues
- **Priority 2 (Important)**: 4 issues
- **Priority 3-4 (Minor/Deferred)**: 2 issues (one is collection)
- **Total**: 9 parent issues

### By Status
- **Existing**: 1 issue (#57)
- **Newly Created**: 8 issues (#109-#116)

### Total Estimated Effort
- **Priority 1**: ~4-5 weeks
- **Priority 2**: ~4-5 weeks
- **Priority 3-4**: ~2-3 weeks + future enhancements
- **Total**: ~10-13 weeks (excluding future enhancements)

---

## Next Steps

### For Each Issue

1. **Review and Refine**
   - Review issue description and acceptance criteria
   - Add any missing details or context
   - Adjust effort estimates based on team capacity

2. **Break Down into Sub-Issues**
   - Create sub-issues for specific implementation tasks
   - Link sub-issues to parent using GitHub's sub-issue feature
   - Assign sub-issues to team members

3. **Prioritize and Schedule**
   - Prioritize within each priority level
   - Schedule work based on team capacity and dependencies
   - Update project board or roadmap

4. **Track Progress**
   - Update issue status as work progresses
   - Document decisions and changes in issue comments
   - Update technical debt register when resolved

### Recommended Order

**Phase 1: Foundation (Critical)**
1. #109 - Database Migration System (blocks PostgreSQL)
2. #57 - PostgreSQL Support (production readiness)
3. #110 - Distributed Rate Limiting (production readiness)

**Phase 2: Reliability (Important)**
4. #112 - Durable Event Queue (observability reliability)
5. #111 - Cache Invalidation API (operational flexibility)

**Phase 3: Deployment (Important)**
6. #113 - Unified HTTP Server (deployment simplification)
7. #114 - Built-in HTTPS Support (security and ease of use)

**Phase 4: Quality of Life (Minor)**
8. #115 - Comprehensive Package Documentation (developer experience)
9. #116 - Optimizations and Future Enhancements (as needed)

---

## Labels Applied

All issues have been labeled with:
- `tech-debt` - Identifies as technical debt
- Priority label: `priority-1`, `priority-2`, `priority-3`, or `priority-4`
- Component labels: `database`, `rate-limiting`, `caching`, `eventbus`, `server`, `admin-ui`, `security`, `documentation`, `enhancement`, `observability`, `redis`

---

## Integration with Documentation

### Updated Documents
- `docs/technical-debt.md` - Added GitHub issue links to each item
- `docs/technical-debt.md` - Added GitHub Issues Summary section

### Cross-References
- Each GitHub issue references `docs/technical-debt.md`
- Each GitHub issue references related documentation
- Technical debt register links back to GitHub issues

---

## Maintenance

### Keeping Issues in Sync
- Update `docs/technical-debt.md` when issue status changes
- Update GitHub issues when technical debt register changes
- Close issues when technical debt is resolved
- Move resolved items to "Resolved Technical Debt" section

### Review Schedule
- **Weekly**: Review Priority 1 issues, update status
- **Monthly**: Review Priority 2-3 issues, adjust priorities
- **Quarterly**: Full review, archive resolved items

---

## Related Documentation

- **Technical Debt Register**: `docs/technical-debt.md` - Detailed technical debt tracking
- **Brownfield Architecture**: `docs/brownfield-architecture.md` - Actual system state
- **PLAN.md**: Project architecture and objectives
- **Documentation Gap Analysis**: `docs/DOCUMENTATION_GAP_ANALYSIS.md` - Documentation review

---

**For questions or updates, see the [Technical Debt Register](technical-debt.md) or individual GitHub issues.**

