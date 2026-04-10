---
layout: default
title: Project Status
nav_order: 1
---

# Raven — Project Status

**Repository:** [ravencloak-org/Raven](https://github.com/ravencloak-org/Raven)  
**Last updated:** April 2026

---

## Milestone Summary

| # | Milestone | Issues | Status |
|---|-----------|--------|--------|
| M1 | [Project Scaffolding](https://github.com/ravencloak-org/Raven/milestone/1) | 9 closed | ✅ Complete |
| M2 | [Core API + Auth](https://github.com/ravencloak-org/Raven/milestone/2) | 10 closed | ✅ Complete |
| M3 | [Ingestion Pipeline](https://github.com/ravencloak-org/Raven/milestone/3) | 13 closed | ✅ Complete |
| M4 | [Chatbot MVP](https://github.com/ravencloak-org/Raven/milestone/4) | 8 closed | ✅ Complete |
| M5 | [Admin Dashboard](https://github.com/ravencloak-org/Raven/milestone/5) | 9 closed | ✅ Complete |
| M6 | [SaaS Infrastructure](https://github.com/ravencloak-org/Raven/milestone/6) | 7 closed | ✅ Complete |
| M7 | [Phase 2 — Voice Agent](https://github.com/ravencloak-org/Raven/milestone/7) | 8 closed | ✅ Complete |
| M8 | [Phase 3 — WebRTC/WhatsApp](https://github.com/ravencloak-org/Raven/milestone/8) | 5 closed | ✅ Complete |
| — | [Raven Pro: Enterprise Connectors](https://github.com/ravencloak-org/Raven/milestone/9) | 7 closed | ✅ Complete |
| M10 | [Edge Optimization](https://github.com/ravencloak-org/Raven/milestone/10) | 3 closed | ✅ Complete |
| — | [MVP Launch](https://github.com/ravencloak-org/Raven/milestone/11) | 14 closed | ✅ Complete |
| M9 | [Intelligence & Retention](https://github.com/ravencloak-org/Raven/milestone/12) | 3 open | 🔵 In planning |

---

## Open Issues — M9: Intelligence & Retention

| Issue | Title |
|-------|-------|
| [#256](https://github.com/ravencloak-org/Raven/issues/256) | feat(cache): semantic response cache — embedding similarity lookup before LLM calls |
| [#257](https://github.com/ravencloak-org/Raven/issues/257) | feat(notifications): post-session conversation email summaries via AWS SES |
| [#258](https://github.com/ravencloak-org/Raven/issues/258) | feat(memory): cross-channel conversation memory and PostHog session tracking |

---

## Merged PRs by Feature Area

### MVP Launch

| PR | Title | Branch |
|----|-------|--------|
| [#251](https://github.com/ravencloak-org/Raven/pull/251) | feat(onboarding): Keycloak realm auto-provisioning and tenant onboarding wizard | `feat/onboarding-keycloak-realm-wizard` |
| [#250](https://github.com/ravencloak-org/Raven/pull/250) | feat(frontend): billing and subscription management UI | `feat/billing-ui-194` |
| [#249](https://github.com/ravencloak-org/Raven/pull/249) | chore: P2 fixes — RLS cleanup, EE stubs, go.mod, dashboards | `chore/p2-fixes` |
| [#248](https://github.com/ravencloak-org/Raven/pull/248) | docs(ci): document required E2E secrets | `docs/e2e-secrets-238` |
| [#247](https://github.com/ravencloak-org/Raven/pull/247) | fix(observability): restore OTel log exporter and HTTP structured logging | `fix/restore-otel-log-exporter` |
| [#246](https://github.com/ravencloak-org/Raven/pull/246) | fix(test): replace t.Skip on webhook retry/dead-letter tests | `fix/webhook-retry-deadletter-tests` |
| [#245](https://github.com/ravencloak-org/Raven/pull/245) | fix(test): harden testutil.NewTestDB migrations path | `fix/testutil-migrations-path` |
| [#244](https://github.com/ravencloak-org/Raven/pull/244) | feat(billing): subscription enforcement — plan limit checks and feature gates | `feat/issue-193-billing-enforcement` |
| [#243](https://github.com/ravencloak-org/Raven/pull/243) | test(e2e): Playwright mobile viewport tests for critical flows | `feat/issue-225-mobile-e2e` |
| [#242](https://github.com/ravencloak-org/Raven/pull/242) | fix(frontend): remove duplicate useMobile import | `fix/duplicate-usemobile-import` |
| [#241](https://github.com/ravencloak-org/Raven/pull/241) | fix: address code review findings on Go backend test suite | `test/go-backend-suite` |

### Mobile-First Responsive Redesign

| PR | Title | Branch |
|----|-------|--------|
| [#229](https://github.com/ravencloak-org/Raven/pull/229) | feat(mobile): form adaptations and touch target sizing | `feat/issue-224-mobile-forms` |
| [#228](https://github.com/ravencloak-org/Raven/pull/228) | feat(mobile): table-to-card views for list pages | `feat/issue-222-mobile-cards` |
| [#227](https://github.com/ravencloak-org/Raven/pull/227) | feat(mobile): responsive modals and bottom-sheet confirms | `feat/issue-223-mobile-modals` |
| [#226](https://github.com/ravencloak-org/Raven/pull/226) | feat(mobile): bottom tab bar navigation | `feat/issue-200-mobile-responsive` |

### Observability & Billing

| PR | Title | Branch |
|----|-------|--------|
| [#209](https://github.com/ravencloak-org/Raven/pull/209) | feat(observability): wire OpenObserve log aggregation and OTEL metrics | `feat/observability-openobserve-otel` |
| [#207](https://github.com/ravencloak-org/Raven/pull/207) | test(go): Go backend unit and integration test suite | `test/go-backend-suite` |
| [#208](https://github.com/ravencloak-org/Raven/pull/208) | test(python): AI worker pytest suite | `test/python-ai-worker-suite` |
| [#206](https://github.com/ravencloak-org/Raven/pull/206) | feat(billing): Hyperswitch + Razorpay payment integration | `feat/billing-hyperswitch-razorpay` |
| [#205](https://github.com/ravencloak-org/Raven/pull/205) | feat(api): per-org rate limiting by subscription tier | `feat/rate-limiting-valkey` |
| [#204](https://github.com/ravencloak-org/Raven/pull/204) | feat(frontend): WhatsApp Business calling UI | `feat/whatsapp-calling-ui` |
| [#203](https://github.com/ravencloak-org/Raven/pull/203) | feat(frontend): voice session management UI | `feat/voice-session-ui` |
| [#202](https://github.com/ravencloak-org/Raven/pull/202) | test(ebpf): privileged eBPF kernel test harness | `test/ebpf-harness` |
| [#201](https://github.com/ravencloak-org/Raven/pull/201) | test(e2e): Playwright E2E test suite — all journeys | `test/playwright-e2e-suite` |

### Edge Optimization (M10)

| PR | Title | Branch |
|----|-------|--------|
| [#191](https://github.com/ravencloak-org/Raven/pull/191) | feat(ebpf): eBPF edge optimization — XDP pre-filtering, kernel observability | `feat/ebpf-edge-optimization` |

### WebRTC / WhatsApp (M8)

| PR | Title | Branch |
|----|-------|--------|
| [#190](https://github.com/ravencloak-org/Raven/pull/190) | feat(whatsapp): WhatsApp Business Calling API | `feat/whatsapp-calling-api-impl` |
| [#189](https://github.com/ravencloak-org/Raven/pull/189) | feat(webhooks): Meta Graph API webhook receiver | `feat/meta-graph-webhooks` |
| [#188](https://github.com/ravencloak-org/Raven/pull/188) | feat(voice): WebRTC session management with LiveKit | `feat/webrtc-session-management` |
| [#187](https://github.com/ravencloak-org/Raven/pull/187) | feat(voice): LiveKit room bridging for WhatsApp calls | `feat/livekit-whatsapp-bridge` |

### Voice Agent (M7)

| PR | Title | Branch |
|----|-------|--------|
| [#186](https://github.com/ravencloak-org/Raven/pull/186) | feat(deploy): EC2 deployment stack with Ansible and Cloudflare Pages | `feat/ec2-deployment-stack` |
| [#184](https://github.com/ravencloak-org/Raven/pull/184) | feat: TTS integration — Cartesia Sonic + Piper | `feat/issue-60-tts-integration` |
| [#183](https://github.com/ravencloak-org/Raven/pull/183) | feat: STT integration — Deepgram + faster-whisper | `feat/issue-59-stt-integration` |
| [#182](https://github.com/ravencloak-org/Raven/pull/182) | fix(voice): nil-request guards, interface repo, not-found handling | `fix/voice-service-review` |
| [#181](https://github.com/ravencloak-org/Raven/pull/181) | feat: ClickHouse QBit vector storage with hybrid RRF retrieval | `feat/issue-118-clickhouse-qbit` |
| [#178](https://github.com/ravencloak-org/Raven/pull/178) | feat: voice session management — tables, lifecycle, transcription storage | `feat/issue-61-voice-sessions` |
| [#177](https://github.com/ravencloak-org/Raven/pull/177) | feat: stranger block/ban, per-user rate limiting, suspicious behavior | `feat/issue-115-stranger-management` |

---

## CI Status

| Check | Status |
|-------|--------|
| Go CI | [![Go CI](https://github.com/ravencloak-org/Raven/actions/workflows/go.yml/badge.svg)](https://github.com/ravencloak-org/Raven/actions/workflows/go.yml) |
| Frontend CI | [![Frontend CI](https://github.com/ravencloak-org/Raven/actions/workflows/frontend.yml/badge.svg)](https://github.com/ravencloak-org/Raven/actions/workflows/frontend.yml) |
| Python CI | [![Python CI](https://github.com/ravencloak-org/Raven/actions/workflows/python.yml/badge.svg)](https://github.com/ravencloak-org/Raven/actions/workflows/python.yml) |
| Security | [![Security](https://github.com/ravencloak-org/Raven/actions/workflows/security.yml/badge.svg)](https://github.com/ravencloak-org/Raven/actions/workflows/security.yml) |
| Coverage | [![codecov](https://codecov.io/gh/ravencloak-org/Raven/branch/main/graph/badge.svg)](https://codecov.io/gh/ravencloak-org/Raven) |

---

## Quick Links

| Resource | Link |
|----------|------|
| Repository | [github.com/ravencloak-org/Raven](https://github.com/ravencloak-org/Raven) |
| Issues | [All open issues](https://github.com/ravencloak-org/Raven/issues) |
| Pull Requests | [All PRs](https://github.com/ravencloak-org/Raven/pulls) |
| Milestones | [All milestones](https://github.com/ravencloak-org/Raven/milestones) |
| Development Guide | [DEVELOPMENT.md](../DEVELOPMENT.md) |
| Contributing | [CONTRIBUTING.md](../CONTRIBUTING.md) |
| Architecture | [docs/wiki/Architecture-Overview.md](wiki/Architecture-Overview.md) |
| Data Model | [docs/wiki/Data-Model.md](wiki/Data-Model.md) |
| Quickstart | [docs/quickstart.md](quickstart.md) |
