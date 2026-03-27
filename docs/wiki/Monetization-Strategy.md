# Monetization Strategy

## BYOK Cost Advantage

Users bring their own LLM API keys (BYOK). Raven pays zero for tokens. Every subscription dollar is nearly pure margin. This enables aggressive pricing while remaining profitable.

## Pricing Tiers

| Tier | Price | Messages | KBs | Documents | Storage | Voice | Key Feature |
|------|-------|----------|-----|-----------|---------|-------|-------------|
| **Free (self-hosted)** | $0 | Unlimited | Unlimited | Unlimited | Unlimited | Unlimited | Full platform, self-hosted |
| **Pro (cloud)** | $29/mo | 5,000 | 5 | 500 | 10 GB | -- | 1 org, 3 workspaces |
| **Business** | $99/mo | 25,000 | 20 | 2,500 | 50 GB | 500 min | Voice agent, white-label |
| **Enterprise** | $299+/mo | 100,000 | Unlimited | 10,000 | 200 GB | 2,000 min | WhatsApp, SSO, SLA |

## Overage Pricing

- Messages: $0.002-$0.004/message (varies by tier)
- Voice: $0.02-$0.03/minute
- Documents: $0.01/document processed

## Open-Core Split

| Feature | Free (self-hosted) | Cloud (paid) |
|---------|-------------------|-------------|
| Core RAG + chatbot | Yes | Yes |
| Multi-tenancy | Yes | Yes |
| Voice agent | Yes | Yes |
| Managed hosting | No | Yes |
| SSO / SAML | No | Enterprise |
| White-label | No | Business+ |
| SLA guarantee | No | Enterprise |
| Audit logs (SOC 2) | No | Enterprise |

## Break-Even Math

| Hosting | Monthly Cost | Customers to Break Even |
|---------|-------------|------------------------|
| Hetzner CCX33 | ~$55/mo | 2 Pro customers |
| AWS (small) | ~$590/mo | 6 Business customers |

**Recommendation:** Start on Hetzner, migrate to AWS at 50+ customers.
