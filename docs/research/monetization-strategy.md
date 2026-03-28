# Raven Platform -- Monetization Strategy

**Date:** 2026-03-27
**Status:** Research document
**Author:** Monetization Research Phase
**Constraint:** Zero upfront infrastructure spend. Break-even is the minimum goal.

---

## Table of Contents

1. [Cost Model -- What Raven Must Cover](#1-cost-model----what-raven-must-cover)
2. [Value Metrics -- What to Monetize](#2-value-metrics----what-to-monetize)
3. [Competitive Pricing Analysis](#3-competitive-pricing-analysis)
4. [Revenue Model Options](#4-revenue-model-options)
5. [Proposed Pricing Tiers](#5-proposed-pricing-tiers)
6. [Break-Even Calculations](#6-break-even-calculations)
7. [Growth Strategy -- First 10 Paying Customers](#7-growth-strategy----first-10-paying-customers)
8. [Implementation Roadmap](#8-implementation-roadmap)

---

## 1. Cost Model -- What Raven Must Cover

### The BYOK Advantage

Raven's single most important economic feature: **users bring their own LLM API keys**. This means Raven does not pay for:
- Embedding generation (OpenAI, Cohere, etc.)
- LLM inference (GPT-4o, Claude, etc.)
- Reranking (Cohere rerank API)
- TTS/STT for voice (ElevenLabs, Deepgram, etc.)

Every competitor that bundles LLM costs into their pricing (Chatbase, CustomGPT, Mendable, Inkeep) has a per-message marginal cost of $0.001-$0.05. Raven's per-message marginal cost is effectively **zero** (just compute for the retrieval pipeline and SSE streaming). This is a structural cost advantage.

### Fixed Infrastructure Costs (Cloud-Managed Offering)

Based on the hardware requirements research, here are Raven's costs at different scales:

#### Phase 0: Pre-Revenue / Development (Target: $0/month)

| Item | Cost | Strategy |
|------|------|----------|
| Domain (.dev or .ai) | $12-50/year | One-time; use Cloudflare for free DNS + SSL |
| Development server | $0 | Use Oracle Cloud free tier (4 OCPU, 24 GB ARM) or local Docker |
| PostgreSQL | $0 | Self-hosted on free-tier VPS |
| PostHog | $0 | Cloud free tier (1M events/month) |
| OpenObserve | $0 | Cloud free tier (200 GB/day) or self-hosted (minimal RAM) |
| Email (SMTP) | $0 | AWS SES free tier (62K emails/month) or Resend free (100/day) |
| SSL/TLS | $0 | Let's Encrypt via Traefik auto-renewal |
| GitHub | $0 | Free for public repos; Actions free tier for CI |
| **Total** | **~$1-4/month** | Domain amortized |

**Oracle Cloud Always-Free Tier breakdown:**
- 4 OCPU ARM Ampere A1 (equivalent to ~4 vCPU)
- 24 GB RAM
- 200 GB block storage
- 10 TB/month outbound data transfer
- This is sufficient to run the entire Raven stack for development and early customers

#### Phase 1: First 10 Paying Customers (~$55-140/month)

| Item | Cost | Notes |
|------|------|-------|
| Hetzner CCX33 (8 dedicated vCPU, 32 GB RAM, 240 GB NVMe) | $55/month | Or Hetzner CPX41 at $28/month (shared vCPU) |
| Domain | ~$1-4/month | Amortized |
| PostHog Cloud | $0 | Free tier covers early usage |
| OpenObserve | $0 | Self-hosted on same server |
| Email (AWS SES) | ~$1/month | Low volume transactional |
| Razorpay | ~2% domestic, ~3% international per transaction | Deducted from revenue |
| **Total (Hetzner CCX33)** | **~$57-60/month** | Comfortable for 10 tenants |
| **Total (Hetzner CPX41, budget)** | **~$30-33/month** | Tight but functional for 10 tenants |

#### Phase 2: 100 Paying Customers (~$200-400/month)

| Item | Cost | Notes |
|------|------|-------|
| Hetzner AX42 dedicated (8-core, 64 GB RAM, 2x512 GB NVMe) | $50/month | Hetzner auction; best value |
| Or: 2x Hetzner CCX33 | $110/month | For redundancy |
| LiveKit server (same machine or separate $28 VPS) | $0-28/month | Voice agent Phase 2 |
| Email | ~$5/month | Higher volume |
| Backups (Hetzner backup space) | ~$5-10/month | Snapshots + off-site |
| **Total** | **~$60-153/month** | Hetzner's pricing is the key enabler |

**Why Hetzner over AWS:** The hardware requirements research shows AWS at $590-700/month for 10 tenants on ECS/RDS. Hetzner delivers equivalent capacity at $55-110/month. For a bootstrapped product, this 5-10x cost difference is the difference between break-even and burning cash.

#### Cost Scaling Summary

| Scale | Hetzner (Self-Managed) | AWS (Managed Services) | Revenue Needed |
|-------|----------------------|----------------------|----------------|
| 10 tenants | $55-60/month | $590/month | $6-59/customer/month |
| 100 tenants | $110-200/month | $2,400/month | $1.10-24/customer/month |
| 1,000 tenants | $400-800/month | $10,000+/month | $0.40-10/customer/month |

**Key insight:** Using Hetzner, break-even at 10 customers requires only ~$6/customer/month. Using AWS, it requires ~$59/customer/month. **Start on Hetzner; migrate to AWS only when you need managed services for reliability at scale.**

---

## 2. Value Metrics -- What to Monetize

### Analysis of Possible Value Metrics

| Metric | Pros | Cons | Verdict |
|--------|------|------|---------|
| **Per message/query** | Directly tied to value; scales with usage; familiar (Chatbase, Mendable model) | Discourages usage; hard to predict costs for customers; feels "metered" | **Use as overage, not primary** |
| **Per document ingested** | Clear unit; reflects processing cost (embedding, chunking) | Discourages uploading; punishes exploration; low perceived value per doc | **Do not use as primary** |
| **Per knowledge base** | Easy to understand; natural expansion metric | Too coarse; one KB can have 10 or 10,000 docs | **Use as tier limit** |
| **Per workspace** | Aligns with team/project structure | Similar to KB -- too coarse alone | **Use as tier limit** |
| **Per organization** | Simplest billing unit; one price per tenant | Doesn't capture usage differences | **Use as base subscription** |
| **Per voice minute** | Directly tied to high-value feature; industry standard (Vapi, Retell) | Complex metering; anxiety for customers | **Use for voice (Phase 2)** |
| **Per API call** | Granular; developer-friendly | Low per-unit value; unpredictable bills | **Do not use** |
| **Per seat (user)** | Predictable; scales with team size | Penalizes collaboration; end-users of chatbot aren't "seats" | **Do not use for chatbot; use for dashboard access** |
| **Feature-gating** | Clear upgrade path; no metering anxiety; free tier drives adoption | Requires careful feature selection; can feel arbitrary | **Primary mechanism** |

### Recommended Value Metric Stack

**Primary:** Feature-gated subscription tiers (per organization)
**Secondary:** Usage limits with soft caps (messages/month, documents, storage)
**Tertiary:** Per-voice-minute for Phase 2 (industry standard, customers expect it)

This is a **hybrid model**: base subscription for features + usage allowances with overage pricing. This matches how the market leaders price (Chatbase, Dify, CustomGPT all use tier + usage limits).

---

## 3. Competitive Pricing Analysis

### Direct Competitors -- Pricing Breakdown

#### RAG Chatbot Platforms

| Platform | Free Tier | Entry Paid | Mid Tier | Top Tier | Pricing Model |
|----------|-----------|-----------|----------|----------|---------------|
| **Chatbase** | 20 msgs/month | $19/mo (2K msgs) | $99/mo (10K msgs) | $399/mo (unlimited) | Per-message tiers |
| **CustomGPT.ai** | None | $89/mo (1K msgs, 10 bots) | $449/mo (10K msgs, 100 bots) | Custom | Per-message + per-bot tiers |
| **DocsBot AI** | None | $19/mo (1K msgs) | $49/mo (5K msgs) | $199/mo (25K msgs) | Per-message tiers |
| **Mendable** | 500 msgs/month | $150/mo (10K msgs) | Custom | Custom | Per-message tiers (developer-focused) |
| **Inkeep** | Small projects | ~$600/mo (growth) | Custom | Custom | Per-conversation (premium) |
| **Dify Cloud** | 200 msgs (sandbox) | $59/mo (professional) | $159/mo (team) | Custom | Feature + message tiers |
| **Flowise Cloud** | None | $35/mo | Custom | Custom | Feature tiers |
| **AnythingLLM Cloud** | None | $6.99/seat/mo | Custom | Custom | Per-seat |
| **Quivr** | Limited | $19.90/mo | Custom | Custom | Feature tiers |

#### Voice AI Platforms

| Platform | Free Tier | Per-Minute Rate | Monthly Minimum | Notes |
|----------|-----------|----------------|-----------------|-------|
| **Vapi** | Trial credits | ~$0.05/min | None (pay-as-you-go) | Includes infra + provider costs |
| **Retell AI** | Trial credits | ~$0.07/min | None | STT + LLM + TTS pipeline |
| **Bland AI** | None | $0.07-$0.12/min | Custom | Enterprise telephony focus |
| **ElevenLabs** | 10K chars/mo | Per-minute (agents) | $5/mo (starter) | TTS-first, agents secondary |
| **PlayHT** | None | Per-character (TTS) | $31.20/mo | TTS-focused |

### Key Pricing Insights

1. **Entry-level chatbot pricing clusters around $19-35/month.** Chatbase ($19), DocsBot ($19), Quivr ($19.90), Flowise ($35). This is the market-validated "hobby/individual" price point.

2. **Mid-tier clusters around $49-99/month.** DocsBot ($49), Chatbase ($99), Dify ($59). This is where small teams and businesses land.

3. **Premium/business clusters around $149-449/month.** Mendable ($150), CustomGPT ($449), Chatbase ($399), Dify ($159), DocsBot ($199).

4. **Message limits define tiers.** The universal pattern is 1K-2K messages for entry, 5K-10K for mid, 10K-25K for business.

5. **Voice is always per-minute, $0.05-$0.12/min.** No subscription-based voice pricing exists in the market.

6. **BYOK changes the economics.** Competitors charging $19/month at 2K messages are paying ~$0.002-$0.01 per message in LLM costs ($4-20/month of their revenue goes to OpenAI). Raven has zero LLM cost per message, meaning the full subscription is margin.

7. **Self-hosting is either free or enterprise-priced.** Dify, Flowise, and AnythingLLM offer free self-hosting. No one charges for self-hosted licenses at the SMB level. Enterprise self-hosted licenses start at $500-2,000+/month.

### Raven's Pricing Sweet Spot

Given that Raven:
- Has zero LLM marginal costs (BYOK)
- Offers more features than Chatbase/DocsBot (multi-tenancy, hybrid search, voice roadmap)
- Is self-hostable (competes with Dify's free self-hosted model)
- Targets a more technical/B2B audience

**The sweet spot is:**
- **Free tier:** Generous enough to match Dify/Chatbase free tiers (drive adoption)
- **Entry paid:** $29/month (above Chatbase $19 because of superior features; below CustomGPT $89)
- **Business:** $99/month (matches Chatbase Standard; significant feature unlock)
- **Enterprise:** $299+/month or custom (multi-tenant API, SLA, priority support)

**Rationale for pricing above Chatbase ($19) at entry level:**
- Raven's hybrid search + reranking delivers better retrieval quality
- Multi-workspace support even at entry tier
- BYOK means customers control their LLM costs (no hidden markup)
- Self-hosting option provides escape hatch that Chatbase cannot offer

---

## 4. Revenue Model Options

### Option A: Pure SaaS Subscription

| Aspect | Detail |
|--------|--------|
| **How it works** | Fixed monthly/annual price per organization. Tier determines feature access and usage limits. |
| **Pros** | Predictable revenue; simple billing; easy to explain; customers know what they pay. |
| **Cons** | Heavy users subsidize light users; no upside from power users; free tier is pure cost. |
| **Best for** | Early stage when simplicity matters; when usage patterns are unknown. |
| **Examples** | Dify ($59/$159), Flowise ($35), AnythingLLM ($6.99/seat) |

### Option B: Pure Usage-Based (Pay Per Use)

| Aspect | Detail |
|--------|--------|
| **How it works** | No subscription. Charge per message, per document ingested, per voice minute. |
| **Pros** | Aligns cost with value; low barrier to entry; scales naturally. |
| **Cons** | Revenue is unpredictable; customers have bill anxiety; hard to forecast; operationally complex (metering). |
| **Best for** | Commodity services where per-unit economics are clear. |
| **Examples** | Vapi ($0.05/min), Retell ($0.07/min), Cohere (per-token) |

### Option C: Hybrid (Base Subscription + Usage Overage) -- RECOMMENDED

| Aspect | Detail |
|--------|--------|
| **How it works** | Fixed monthly subscription includes generous usage allowance. Overage charged per unit above the allowance. |
| **Pros** | Predictable base revenue; captures value from power users; generous allowance reduces bill anxiety; aligns with how most SaaS prices. |
| **Cons** | Slightly more complex billing; need usage metering infrastructure. |
| **Best for** | Platforms with variable usage patterns and both light/heavy users. |
| **Examples** | Chatbase (tiers + overage), PostHog (free allowance + pay-as-you-go), Vercel (hobby free + usage) |

### Option D: Open-Core (Free Self-Hosted + Paid Cloud Features)

| Aspect | Detail |
|--------|--------|
| **How it works** | Core platform is open-source and free to self-host. Cloud-managed version adds convenience features. Certain "enterprise" features only available in paid edition. |
| **Pros** | Massive developer adoption; community contributions; trust through transparency; viral distribution. |
| **Cons** | Revenue conversion is low (typically 1-5% of users pay); must carefully choose what's free vs. paid; support burden from free users. |
| **Best for** | Developer tools where adoption is the primary moat. |
| **Examples** | Dify (free self-hosted, paid cloud), GitLab (CE vs EE), PostHog (open-core + cloud) |

### Option E: Marketplace / Premium Add-Ons

| Aspect | Detail |
|--------|--------|
| **How it works** | Base platform is subscription-priced. Premium integrations, templates, or connectors are sold separately or as add-ons. |
| **Pros** | Additional revenue stream; partners can build and sell; extensible ecosystem. |
| **Cons** | Requires ecosystem scale to be meaningful; complex to manage; early-stage distraction. |
| **Best for** | Mature platforms with established user base. |
| **Examples** | Shopify App Store, WordPress plugins, Slack Marketplace |

### Recommended Model: C + D Hybrid

**Combine Open-Core (D) with Hybrid Subscription + Usage (C):**

1. **Self-hosted (startups / < $5M ARR):** Free, core features only
2. **Self-hosted (enterprise / > $5M ARR):** Paid license required for `ee/` features
3. **Cloud-managed:** Hybrid subscription with usage allowances and overage

This mirrors the Dify/GitLab/PostHog model and is the dominant pattern for developer-facing infrastructure tools in 2026.

#### Open-Core Enforcement: `ee/` Directory Model

Enterprise features live in a separate `ee/` directory with a **Raven Enterprise License** (source-available, NOT open-source). The core platform remains Apache 2.0.

```
raven/
├── internal/           ← Apache 2.0 (truly open source)
├── ee/                 ← Raven Enterprise License (source-available)
│   ├── LICENSE          → Production use requires a valid license key
│   ├── lead/            → Lead intelligence (profiles, scoring, CRM export)
│   ├── webhooks/        → Event webhooks (lead.generated, escalation, etc.)
│   ├── connectors/      → Connector UI + data catalog integration
│   ├── security/        → Advanced WAF rules, DDoS, per-user blocking
│   ├── audit/           → Audit logs + compliance reports
│   ├── sso/             → SAML/OIDC enterprise SSO
│   └── analytics/       → Individual lead profiles + advanced reporting
```

**License enforcement:** Signed JWT license key validated at startup. Contains org_id, tier, expiry, feature list, HMAC signature (only Raven can generate). `ee/` features return 403 without valid license.

**Precedent:** GitLab ($500M+ ARR), PostHog ($100M+ valuation), Sentry ($100M+ ARR), Dify all use this model. It is legally enforceable — the `ee/` license prohibits production use without payment. Companies above the revenue threshold that strip the license check are in legal violation.

**What's free (self-hosted, Apache 2.0 core):**
- Full RAG pipeline (ingestion, hybrid search, reranking)
- Embeddable chatbot widget
- Multi-tenant hierarchy (org > workspace > KB)
- BYOK LLM support
- Voice agent (Phase 2)
- WebRTC (Phase 3)
- API access
- Basic analytics (aggregate stats only)
- Basic rate limiting + security headers
- Manual file upload + URL scraping
- YAML-based connector config (DIY Airbyte setup)

**What's paid (`ee/` — requires license for cloud-managed OR self-hosted):**

*Lead Intelligence (Business/Enterprise):*
- Individual lead profiles (contact info, conversation history, intent signals)
- Lead engagement scoring
- CRM export (Salesforce, HubSpot) via API
- Free tier sees aggregate stats only; paid sees per-user data

*Event Webhooks (Business/Enterprise):*
- `lead.generated` — fires on purchase intent or contact info capture
- `lead.qualified` — fires when engagement crosses scoring threshold
- `conversation.escalation` — fires when bot can't answer, human needed
- Free tier: no webhooks

*Stranger User Management (Business/Enterprise):*
- Block/ban specific users (by IP, fingerprint, session ID)
- Per-user rate limiting (not just global)
- Suspicious behavior flagging
- Free tier: basic global rate limiting only

*Advanced Security (Enterprise):*
- Custom WAF-style rules (geo-blocking, request pattern rules)
- DDoS protection (eBPF XDP layer at kernel level)
- IP allowlisting/denylisting
- Abuse detection and alerting

*Enterprise Data (Enterprise):*
- Airbyte connector UI + managed connectors
- Data catalog integration (Snowflake tags, dbt, DataHub, Glue)
- ClickHouse + QBit vector scaling
- SSO (SAML/OIDC beyond Keycloak basic)
- Audit logs + compliance reports
- Multi-org / white-label
- Managed hosting, auto-scaling, backups, DR
- Custom domain
- Uptime SLA guarantees
- Priority support

---

## 5. Proposed Pricing Tiers

### Cloud-Managed Pricing

#### Tier 0: Free (Community)

| Feature | Limit |
|---------|-------|
| **Price** | $0/month |
| **Purpose** | Developer exploration, proof of concept, small personal projects |
| Knowledge bases | 1 |
| Workspaces | 1 |
| Documents | 50 |
| Storage | 100 MB |
| Messages (chatbot queries) | 500/month |
| Embeddable chatbot widgets | 1 |
| Voice agent | Not included |
| API access | Not included |
| Users (dashboard) | 1 |
| LLM providers | BYOK (any) |
| Search | Hybrid (vector + BM25) |
| Support | Community (GitHub Discussions) |
| Data retention | 30 days chat history |

**Rationale:** 500 messages/month is generous enough to build and test a chatbot but restrictive enough that any real usage hits the limit. 50 documents allows meaningful testing. One KB and one workspace prevent free-tier abuse for multi-tenant scenarios. This matches Chatbase's free tier spirit (20 msgs is too stingy; 500 is proven by Mendable).

#### Tier 1: Pro ($29/month, or $290/year -- save $58)

| Feature | Limit |
|---------|-------|
| **Price** | $29/month |
| **Target** | Freelancers, small businesses, indie developers |
| Knowledge bases | 5 |
| Workspaces | 3 |
| Documents | 500 |
| Storage | 2 GB |
| Messages | 5,000/month |
| Overage | $0.004/message ($4 per 1,000) |
| Embeddable chatbot widgets | 5 (custom branding) |
| Voice agent (Phase 2) | 60 minutes/month included |
| Voice overage | $0.03/minute |
| API access | Full REST API |
| Users (dashboard) | 3 |
| LLM providers | BYOK (any) |
| Search | Hybrid + reranking |
| Custom chatbot appearance | Yes |
| Remove "Powered by Raven" | Yes |
| Support | Email (48h response) |
| Data retention | 90 days chat history |
| Analytics | Basic (query count, popular topics) |

**Rationale:** $29/month positions Raven above commodity chatbot builders ($19) and signals quality. 5,000 messages/month is generous for small businesses (most Chatbase $19 users get 2,000). The BYOK model means this is nearly pure margin. Voice minutes at 60/month are a taste of Phase 2 functionality.

#### Tier 2: Business ($99/month, or $990/year -- save $198)

| Feature | Limit |
|---------|-------|
| **Price** | $99/month |
| **Target** | Growing businesses, agencies, SaaS companies embedding Raven |
| Knowledge bases | 25 |
| Workspaces | 10 |
| Documents | 5,000 |
| Storage | 20 GB |
| Messages | 25,000/month |
| Overage | $0.003/message ($3 per 1,000) |
| Embeddable chatbot widgets | 25 |
| Voice agent (Phase 2) | 300 minutes/month included |
| Voice overage | $0.025/minute |
| API access | Full REST API + webhooks |
| Users (dashboard) | 10 |
| Multi-tenant API | Yes (create KBs programmatically) |
| LLM providers | BYOK (any) |
| Search | Hybrid + reranking + analytics |
| Custom chatbot appearance | Yes (full CSS control) |
| Custom domain (CNAME) | Yes |
| White-label (remove all Raven branding) | Yes |
| SSO (Google, GitHub, SAML) | Yes |
| Support | Email (24h response) + Slack channel |
| Data retention | 1 year chat history |
| Analytics | Advanced (conversation analytics, knowledge gaps, user satisfaction) |
| Webhooks | Yes (new message, document processed, etc.) |

**Rationale:** $99/month matches Chatbase Standard and is the volume tier. Agencies managing multiple client chatbots need 25 KBs and the multi-tenant API. White-labeling and custom domains are high-value features that cost Raven nothing to provide but are worth significant money to agencies.

#### Tier 3: Enterprise ($299/month base, or custom pricing)

| Feature | Limit |
|---------|-------|
| **Price** | Starting at $299/month (custom pricing above) |
| **Target** | Large companies, regulated industries, platform builders |
| Knowledge bases | Unlimited |
| Workspaces | Unlimited |
| Documents | Unlimited |
| Storage | 100 GB (expandable) |
| Messages | 100,000/month included |
| Overage | $0.002/message ($2 per 1,000) |
| Embeddable chatbot widgets | Unlimited |
| Voice agent (Phase 2) | 1,000 minutes/month included |
| Voice overage | $0.02/minute |
| API access | Full REST API + webhooks + bulk operations |
| Users (dashboard) | Unlimited |
| Multi-tenant API | Yes (full org/workspace/KB management) |
| LLM providers | BYOK (any) + Raven-managed option |
| Search | Hybrid + reranking + knowledge graph (Phase 4) |
| Dedicated infrastructure | Optional (isolated PostgreSQL, dedicated compute) |
| Custom domain | Yes |
| White-label | Yes |
| SSO (SAML/OIDC) | Yes |
| Audit logs | Yes |
| RBAC (fine-grained roles) | Yes |
| SOC 2 compliance report | Available |
| GDPR data processing agreement | Included |
| Support | Priority email (4h response) + dedicated Slack + quarterly review |
| Data retention | Custom (up to unlimited) |
| Analytics | Full suite + custom dashboards + data export |
| SLA | 99.9% uptime guarantee |
| Onboarding | Dedicated onboarding session |

**Rationale:** $299/month is the entry point for enterprise; large deployments will be custom-priced ($500-2,000+/month). The key enterprise differentiators (audit logs, RBAC, SLA, compliance) cost almost nothing to build but are table-stakes for procurement processes.

### Self-Hosted Pricing

| Tier | Price | Who | What's Included |
|------|-------|-----|----------------|
| **Community** | Free | Startups, students, companies < $5M ARR | Apache 2.0 core features, no limits, community support only |
| **Self-Hosted Pro** | $99/month | Growing companies | Core + lead intelligence, webhooks, connectors, per-user blocking. Stable releases + patch support. |
| **Self-Hosted Enterprise** | $499+/month | Companies > $5M ARR, regulated industries | All `ee/` features (SSO, audit, security rules, data catalog integration, ClickHouse + QBit). Priority patches + dedicated support channel + SLA. |

**Revenue threshold enforcement:** Companies below $5M ARR can use Self-Hosted Pro features via a startup program (free or discounted). Above $5M ARR, the Raven Enterprise License terms require a paid license for `ee/` features. This is enforced legally (license terms), not just technically (license key). Precedent: Sentry BSL, AG Grid.

**Rationale:** The open-source community edition is the adoption funnel. Self-Hosted Pro captures growing companies who want lead intelligence and webhooks but prefer to run their own infrastructure. Self-Hosted Enterprise captures large companies with compliance requirements. The `ee/` directory model ensures that upgrading from Community → Pro → Enterprise is a license key change, not a migration.

### Pricing Summary Table — Cloud-Managed

| | Free | Pro | Business | Enterprise |
|---|------|-----|----------|-----------|
| **Monthly** | $0 | $29 | $99 | $299+ |
| **Annual** | $0 | $290 | $990 | Custom |
| **Messages** | 500 | 5,000 | 25,000 | 100,000 |
| **KBs** | 1 | 5 | 25 | Unlimited |
| **Docs** | 50 | 500 | 5,000 | Unlimited |
| **Voice mins** | -- | 60 | 300 | 1,000 |
| **Widgets** | 1 | 5 | 25 | Unlimited |
| **Users** | 1 | 3 | 10 | Unlimited |
| **API** | -- | Yes | Yes + webhooks | Full |
| **Lead intelligence** | -- | -- | Individual profiles + scoring | + CRM export |
| **Event webhooks** | -- | -- | lead.generated, escalation | All events + custom |
| **User management** | -- | -- | Block/ban users | + abuse detection |
| **Security** | Basic | Basic | + per-user rate limits | + custom WAF rules, DDoS |
| **Connectors (Airbyte)** | -- | -- | 3 connectors | Unlimited + catalog integration |
| **White-label** | -- | -- | Yes | Yes |
| **SSO** | -- | -- | Yes | Yes |
| **Audit logs** | -- | -- | -- | Yes |
| **Support** | Community | Email | Email + Slack | Priority + SLA |

### Pricing Summary Table — Self-Hosted

| | Community | Self-Hosted Pro | Self-Hosted Enterprise |
|---|-----------|----------------|----------------------|
| **Monthly** | $0 | $99 | $499+ |
| **Who** | Startups, < $5M ARR | Growing companies | Enterprises, > $5M ARR |
| **Core features** | All | All | All |
| **`ee/` features** | -- | Lead intel, webhooks, connectors, user mgmt | All (+ SSO, audit, security, catalog) |
| **Connector config** | YAML (DIY) | YAML + UI | UI + data catalog integration |
| **Updates** | Community releases | Stable + patches | Priority patches |
| **Support** | Community | Email | Dedicated + SLA |
| **License** | Apache 2.0 | Raven Enterprise License | Raven Enterprise License |

---

## 6. Break-Even Calculations

### Scenario A: 10 Customers on Hetzner CCX33 ($55/month)

**Fixed costs:**
| Item | Monthly Cost |
|------|-------------|
| Hetzner CCX33 | $55 |
| Domain (amortized) | $2 |
| AWS SES (email) | $1 |
| **Total** | **$58/month** |

**Revenue scenarios (10 customers):**

| Customer Mix | Monthly Revenue | Payment Fees (~2-3%) | Net Revenue | Profit/Loss |
|-------------|----------------|--------------------------|-------------|-------------|
| 10x Pro ($29) | $290 | $11.41 | $278.59 | **+$220.59** |
| 5x Pro + 5x Free | $145 | $5.71 | $139.29 | **+$81.29** |
| 3x Business + 7x Free | $297 | $9.57 | $287.43 | **+$229.43** |
| 2x Pro + 1x Business | $157 | $5.57 | $151.43 | **+$93.43** |
| 10x Free | $0 | $0 | $0 | **-$58** |

**Break-even point:** 2 Pro customers ($58 revenue) covers costs. With 10 paying customers at any paid tier, Raven is profitable from day one on Hetzner.

**Minimum price per customer for break-even with 10 customers:**
- $58 / 10 = $5.80/customer/month (before payment gateway fees)
- With payment fees: ~$6.27/customer/month (varies by gateway)

This means even a hypothetical $7/month "Starter" tier would break even at 10 customers. The proposed $29 Pro tier provides **5x headroom** above break-even.

### Scenario B: 10 Customers on Hetzner CPX41 ($28/month, Budget Option)

| Item | Monthly Cost |
|------|-------------|
| Hetzner CPX41 | $28 |
| Domain + Email | $3 |
| **Total** | **$31/month** |

**Break-even:** Just 2 Pro customers ($58 revenue - ~$2.38 fees = $55.62 net). Even 1 Business customer ($99 - ~$3.17 = $95.83 net) covers costs 3x over.

### Scenario C: Scaling to 100 Customers

**Assumption:** Upgrade to Hetzner AX42 dedicated ($50/month) + backup VPS ($28/month) = $78/month.

**Customer mix assumption (100 total):**
| Tier | Count | MRR | Note |
|------|-------|-----|------|
| Free | 60 | $0 | Funnel / community |
| Pro ($29) | 25 | $725 | Core revenue |
| Business ($99) | 12 | $1,188 | High-value segment |
| Enterprise ($299) | 3 | $897 | Anchor accounts |
| **Total** | **100** | **$2,810** | |

**Costs at 100 customers:**
| Item | Monthly Cost |
|------|-------------|
| Hetzner AX42 + backup | $78 |
| Domain + Email | $10 |
| payment gateway fees (~3.15% effective) | $88.52 |
| **Total** | **$176.52** |

**Net profit: $2,633.48/month ($31,602/year)**

This does not include the founder's time, which is the real cost at this stage. But the infrastructure economics are strongly positive.

### Scenario D: What If AWS Is Required? (10 Customers)

Per hardware requirements research, AWS costs $590/month for Tier 1 (10 tenants).

| Customer Mix | Monthly Revenue | Payment Fees | Net Revenue | Profit/Loss |
|-------------|----------------|-------------|-------------|-------------|
| 10x Pro ($29) | $290 | $11.41 | $278.59 | **-$311.41** |
| 10x Business ($99) | $990 | $31.71 | $958.29 | **+$368.29** |
| 5x Business + 5x Pro | $640 | $21.56 | $618.44 | **+$28.44** |

**Verdict:** On AWS, you need at least 5 Business-tier customers just to break even. This is why Hetzner is essential for bootstrapping. AWS is only viable once you have 50+ paying customers or enterprise contracts.

### Revenue Per Customer Sensitivity

| Metric | Hetzner ($58/mo costs) | AWS ($590/mo costs) |
|--------|----------------------|---------------------|
| Break-even at 5 customers | $12.30/customer | $125.30/customer |
| Break-even at 10 customers | $6.27/customer | $62.69/customer |
| Break-even at 25 customers | $2.53/customer | $25.14/customer |
| Break-even at 50 customers | $1.27/customer | $12.62/customer |

---

## 7. Growth Strategy -- First 10 Paying Customers

### Phase 0: Pre-Launch (Months 1-2)

**Goal:** Build in public, create awareness, validate demand.

#### 7.1 Open-Source Community Launch

1. **GitHub repository with excellent README** -- Include animated GIF of the chatbot widget, one-command Docker Compose setup, architecture diagram. Target 100 stars in the first week.

2. **"Show HN" on Hacker News** -- Title: "Raven -- Open-source multi-tenant RAG platform with embeddable chatbot (Go + Python, self-hostable)" -- The self-hostable + BYOK angle resonates strongly on HN.

3. **Product Hunt launch** -- Prepare assets, screenshots, demo video. Target top 5 of the day. Schedule for Tuesday/Wednesday (highest engagement days).

4. **r/selfhosted, r/LocalLLaMA, r/ChatGPT, r/artificial** -- Reddit is where self-hosted AI tool enthusiasts congregate. Cross-post the GitHub repo with a "I built this" narrative.

5. **Dev.to / Hashnode / Medium technical articles:**
   - "Building a Multi-Tenant RAG Platform with Go and pgvector"
   - "Why Hybrid Search (BM25 + Vector) Beats Pure Vector Search"
   - "Self-Hosted AI Chatbot: From PDF Upload to Embedded Widget in 5 Minutes"

6. **Discord / Slack community** -- Create a Raven community server. Active communities around open-source tools are the highest-converting free channels.

#### 7.2 Content Marketing (The Knowledge-Base Irony)

Raven is a knowledge-base platform. **Use Raven to power Raven's own documentation chatbot.** This is simultaneously:
- A product demo (visitors see the chatbot working live)
- Content marketing (the docs are searchable and findable)
- Social proof ("eat your own dogfood")

Embed the `<raven-chat>` widget on the Raven documentation site. Every visitor interaction is a demo.

#### 7.3 Direct Outreach to Early Adopters

**Target segments for first 10 customers:**

| Segment | Why They Need Raven | Where to Find Them | Willingness to Pay |
|---------|--------------------|--------------------|-------------------|
| **SaaS companies with documentation** | Want AI chatbot for docs without building it | ProductHunt, Indie Hackers, MicroConf | High -- it is a support cost reducer |
| **Digital agencies** | Build chatbots for clients; need white-label multi-tenant | Agency directories, Clutch.co, Upwork | High -- they resell to clients |
| **E-commerce stores** | Product FAQ chatbot; reduce support tickets | Shopify app store (future), e-commerce forums | Medium -- price-sensitive |
| **Developer tool companies** | Technical docs chatbot (Mendable/Inkeep alternative) | GitHub, Dev.to, HN | High -- they already pay for Mendable |
| **Consultants / freelancers** | Offer AI chatbot as a service to clients | Freelance communities, LinkedIn | Medium -- value white-labeling |
| **Internal knowledge management** | Company wiki chatbot for employees | IT departments, Knowledge Management communities | High -- enterprise budgets |

**Tactics:**
- Cold email 50 SaaS companies that currently use Chatbase/Mendable (check their websites for chatbot widgets). Offer a free migration + 3 months Pro tier.
- Partner with 3-5 digital agencies. Offer them Business tier free for 6 months in exchange for case studies.
- Post in Indie Hackers with a "I'll set up a RAG chatbot for your SaaS docs for free" offer. Convert successful setups to paid.

### Phase 1: First Revenue (Months 3-4)

#### 7.4 Open-Source to Cloud Conversion Funnel

```
GitHub stars / Docker pulls (free self-hosted users)
    |
    | 5-10% try cloud
    v
Free tier signups (explore, POC)
    |
    | 10-20% convert to paid
    v
Pro tier ($29/month) -- first revenue
    |
    | 20-30% upgrade over time
    v
Business tier ($99/month)
    |
    | Enterprise conversations
    v
Enterprise ($299+/month)
```

**Conversion rate benchmarks from open-source companies:**
- Dify: ~2-3% of self-hosted users convert to cloud
- PostHog: ~3-5% of users convert to paid
- GitLab: ~1-2% of CE users become EE customers
- Supabase: ~3-4% free-to-paid conversion

**Target:** 1,000 GitHub stars -> 100 cloud free tier signups -> 10 paying customers (10% conversion from free to paid is aggressive but achievable with a good product and proactive onboarding).

#### 7.5 Pricing Page Optimization

- Show the pricing table prominently on the landing page
- Include a "Calculate your cost" tool that shows BYOK savings vs. competitors
- Example: "With Chatbase at $99/month, 10K messages cost $99 + your OpenAI API costs are hidden. With Raven at $99/month, 25K messages + you only pay OpenAI directly (typically $5-15/month for API keys). Total: $99 vs $99+hidden markup."
- Offer annual billing at a 2-month discount (17% off) to improve cash flow

#### 7.6 Referral Program

- Offer 20% revenue share for 12 months to anyone who refers a paying customer
- This is aggressive but cash-efficient (you pay only on success, no upfront cost)
- Target: Agency partners who embed Raven for their clients

### Phase 2: Scale to 100 Customers (Months 5-12)

#### 7.7 Voice Agent Launch (Phase 2 Differentiator)

When voice agent launches:
- Announce on all channels (HN, PH, Reddit, community)
- Position as "The first open-source platform where your chatbot and voice agent share the same knowledge base"
- Offer early access to voice at a discounted rate for existing customers
- This is the feature that moves Raven from "another chatbot builder" to "unique multi-channel knowledge platform"

#### 7.8 Integration Marketplace Seeds

- Zapier / Make.com integration (connects Raven to 5,000+ tools)
- Slack bot (query your knowledge base from Slack)
- WordPress plugin (one-click chatbot embed)
- Shopify app (product FAQ chatbot)

Each integration opens a new distribution channel.

#### 7.9 Case Studies and Social Proof

- After first 5 paying customers, create detailed case studies
- "How [Company] Reduced Support Tickets by 40% with Raven"
- Video testimonials from agency partners
- Public metrics: "Serving X queries/month across Y organizations"

---

## 8. Implementation Roadmap

### Billing Implementation Priority

| Priority | Feature | Tool | Effort |
|----------|---------|------|--------|
| **P0 (MVP)** | Deploy Hyperswitch payment orchestration | Hyperswitch (Docker self-hosted) | 1-2 days |
| **P0 (MVP)** | Configure Razorpay connector (domestic INR) | Hyperswitch + Razorpay | 1 day |
| **P0 (MVP)** | Configure Paddle connector (international MoR) | Hyperswitch + Paddle | 1 day |
| **P0 (MVP)** | Smart routing rules (domestic→Razorpay, international→Paddle) | Hyperswitch routing config | 1 day |
| **P0 (MVP)** | Unified webhook handler for payment events | Go API + Hyperswitch webhooks | 1-2 days |
| **P0 (MVP)** | Org <-> Hyperswitch Customer mapping | Go API + PostgreSQL | 1 day |
| **P0 (MVP)** | Plan/tier enforcement (feature gates) | Go middleware | 2-3 days |
| **P1 (v1.0)** | Usage metering (message count, document count) | Go API counters -> PostgreSQL | 2-3 days |
| **P1 (v1.0)** | Usage-based overage billing | Hyperswitch + gateway add-ons | 2-3 days |
| **P1 (v1.0)** | Self-service plan management UI | Vue.js + Hyperswitch hosted page | 1-2 days |
| **P2 (v1.1)** | Annual billing toggle | Hyperswitch plan switching | 1 day |
| **P2 (v1.1)** | Voice minute metering (Phase 2) | Go API + LiveKit hooks | 2 days |
| **P2 (v1.1)** | Cost observability dashboard | Hyperswitch built-in | 1 day |
| **P3 (future)** | Lago/OpenMeter for advanced metering | Lago API | 1-2 weeks |
| **P3 (future)** | Enterprise invoice billing (net-30) | Hyperswitch invoicing | 1-2 days |
| **P3 (future)** | Additional gateways (Stripe, Adyen, PayPal) | Hyperswitch connectors | 1 day each |

> **Payment Orchestration via Hyperswitch:** Instead of integrating directly with Razorpay or Stripe, Raven uses Hyperswitch (open-source, Apache 2.0, by Juspay — 41.8K GitHub stars, Rust) as a payment orchestration layer. One API integration, multiple gateway backends. Smart routing picks the cheapest or highest-success-rate gateway per transaction. Automatic failover if any gateway is down. Razorpay handles domestic India (UPI, RuPay, netbanking), Paddle handles international (Visa/MC + Merchant of Record for tax compliance). Additional gateways can be added via Hyperswitch connectors without code changes.

### Usage Metering Architecture

```
[Chatbot Query] --> [Go API] --> increment counter --> [PostgreSQL: usage_records]
                                                         |
[Document Upload] --> [Go API] --> increment counter ----+
                                                         |
[Voice Session] --> [LiveKit webhook] --> log minutes ---+
                                                         |
                    [Cron: hourly] --> report to Hyperswitch usage API
                                         |
                    [Cron: daily] --> check limits --> soft-cap warnings (email)
                                         |
                    [Cron: monthly] --> Hyperswitch invoice with overages
```

### Feature Gate Implementation

Feature gates should be enforced at the Go API middleware layer:

```
Request -> Auth middleware -> Plan middleware -> Handler
                               |
                               +-- Check org's plan tier
                               +-- Check feature access (is voice enabled?)
                               +-- Check usage limits (messages remaining?)
                               +-- If over limit: return 429 with upgrade prompt
                               +-- If feature not in plan: return 403 with plan info
```

Store plan definitions in PostgreSQL, cached in Valkey. Hyperswitch unified webhooks update the plan when payment succeeds/fails (regardless of which gateway processed it). License keys for `ee/` features are generated on `subscription.activated` webhook.

---

## Appendix A: Competitor Quick-Reference Pricing

| Competitor | Free | Starter | Mid | Top | Model | Voice? | Self-Host? | BYOK? |
|-----------|------|---------|-----|-----|-------|--------|-----------|-------|
| Chatbase | 20 msg | $19 (2K) | $99 (10K) | $399 (unlimited) | Subscription + limits | No | No | No |
| CustomGPT | None | $89 (1K) | $449 (10K) | Custom | Subscription + limits | No | No | No |
| DocsBot | None | $19 (1K) | $49 (5K) | $199 (25K) | Subscription + limits | No | No | No |
| Mendable | 500 msg | $150 (10K) | Custom | Custom | Per-conversation | No | No | No |
| Inkeep | Yes | ~$600 | Custom | Custom | Per-conversation | No | No | No |
| Dify | 200 msg | $59 | $159 | Custom | Subscription + features | No | Yes* | Yes |
| Flowise | None | $35 | Custom | Custom | Subscription | No | Yes | Yes |
| AnythingLLM | None | $6.99/seat | Custom | Custom | Per-seat | No | Yes | Yes |
| Vapi | Trial | $0.05/min | Volume | Custom | Per-minute | Yes | No | Yes |
| Retell | Trial | $0.07/min | Volume | Custom | Per-minute | Yes | No | Yes |
| **Raven** | **500 msg** | **$29 (5K)** | **$99 (25K)** | **$299+ (100K)** | **Hybrid** | **Yes (P2)** | **Yes** | **Yes** |

## Appendix B: Annual Revenue Projections

### Conservative Scenario (Slow Growth)

| Month | Free Users | Pro | Business | Enterprise | MRR | ARR |
|-------|-----------|-----|----------|-----------|-----|-----|
| 3 | 20 | 2 | 0 | 0 | $58 | $696 |
| 6 | 60 | 8 | 2 | 0 | $430 | $5,160 |
| 9 | 120 | 15 | 5 | 1 | $1,229 | $14,748 |
| 12 | 200 | 25 | 10 | 2 | $2,313 | $27,756 |

### Moderate Scenario (Voice Agent Launches at Month 6)

| Month | Free Users | Pro | Business | Enterprise | MRR | ARR |
|-------|-----------|-----|----------|-----------|-----|-----|
| 3 | 30 | 3 | 1 | 0 | $186 | $2,232 |
| 6 | 100 | 12 | 5 | 1 | $1,142 | $13,704 |
| 9 | 250 | 30 | 12 | 3 | $3,855 | $46,260 |
| 12 | 500 | 50 | 25 | 5 | $6,420 | $77,040 |

### Key Financial Milestones

| Milestone | Conservative | Moderate | Requirement |
|-----------|-------------|----------|-------------|
| Break-even (infra costs covered) | Month 2 | Month 1 | 2 Pro customers |
| $1,000 MRR | Month 8 | Month 5 | ~15 paid customers |
| $5,000 MRR | Month 14 | Month 10 | ~50 paid customers |
| $10,000 MRR | Month 20+ | Month 13 | ~100 paid customers |
| Founder full-time viable ($5K/mo) | Month 14 | Month 10 | $5K+ profit after costs |

---

## Appendix C: Key Decisions Summary

| Decision | Recommendation | Rationale |
|----------|---------------|-----------|
| **Primary revenue model** | Hybrid (subscription + usage overage) | Industry standard; predictable base + upside from power users |
| **Distribution model** | Open-core with `ee/` directory split | Maximizes adoption; proven by GitLab/PostHog/Sentry/Dify |
| **License enforcement** | Signed JWT license key + Raven Enterprise License on `ee/` | Legally enforceable; revenue-threshold gate for startups |
| **Payment orchestration** | Hyperswitch (Razorpay domestic + Paddle international) | Open-source, gateway-independent, smart routing + failover. Stripe not available in India. |
| **Starting infrastructure** | Hetzner (not AWS) | 5-10x cheaper; break-even with 2 customers vs. 5-6 |
| **Entry price point** | $29/month (cloud), $99/month (self-hosted pro) | Above Chatbase ($19) to signal quality; below CustomGPT ($89) for accessibility |
| **Free tier generosity** | 500 messages, 50 docs, 1 KB | Generous enough for testing; forces upgrade for any real usage |
| **Self-hosted free threshold** | Companies < $5M ARR | Startups get core free; enterprise pays. Legal + license key enforcement |
| **Voice pricing (Phase 2)** | Included minutes + per-minute overage | Matches Vapi/Retell model but with included minutes to reduce friction |
| **Lead intelligence** | Business tier+ (paid) | Key monetization lever — turns Raven from cost center to revenue generator for clients |
| **Event webhooks** | Business tier+ (paid) | lead.generated, escalation events — high value for sales-driven clients |
| **Stranger user management** | Business tier+ (paid) | Block/ban, per-user rate limiting — security upsell |
| **Advanced security (WAF/DDoS)** | Enterprise only (paid) | eBPF XDP layer + custom rules — justifies enterprise pricing |
| **Enterprise connectors** | Enterprise only (paid) | Airbyte UI + data catalog integration — on-prem differentiator |
| **First customers** | SaaS docs teams + digital agencies | Highest willingness to pay; clear pain point; referenceable logos |
| **Enterprise self-hosted** | $499+/month | Captures enterprise value without cloud hosting costs |

---

*This document should be revisited after the first 10 paying customers to validate assumptions and adjust pricing based on actual usage patterns and customer feedback.*

*Updated 2026-03-28: Added `ee/` directory enforcement model, self-hosted tiered pricing (Community/Pro/Enterprise), lead intelligence, event webhooks, stranger user management, advanced security features, enterprise connector system, and revenue-threshold gating for self-hosted.*
