import { Button } from '@/components/Button'
import { Container } from '@/components/Container'

const plans = [
  {
    name: 'Self-Hosted',
    price: '₹0',
    cadence: 'forever',
    description: 'Run on your own hardware. No upsell wall, no telemetry.',
    cta: { label: 'Self-host in 5 min', href: '/self-host', variant: 'solid' as const, color: 'ink' as const },
    features: [
      'Unlimited workspaces, users, documents',
      'Voice + chat + channel ingestion',
      'pgvector + BM25 hybrid search',
      'Bring your own models (Ollama, OpenAI, Anthropic, …)',
      'Community support on GitHub',
    ],
  },
  {
    name: 'Cloud Starter',
    price: '₹X',
    cadence: 'per seat / month',
    description: 'Managed by us, in the region of your choice.',
    cta: { label: 'Talk to us', href: 'mailto:hello@ravencloak.org', variant: 'solid' as const, color: 'accent' as const },
    features: [
      'Everything in Self-Hosted',
      'Managed PostgreSQL, Valkey, LiveKit',
      '99.9% uptime SLA',
      'Email support',
    ],
    highlight: true,
  },
  {
    name: 'Cloud Pro',
    price: '₹Y',
    cadence: 'per seat / month',
    description: 'For teams that need SSO, audit logs, and priority support.',
    cta: { label: 'Talk to us', href: 'mailto:hello@ravencloak.org', variant: 'outline' as const, color: 'ink' as const },
    features: [
      'Everything in Cloud Starter',
      'SSO (SAML, OIDC)',
      'Immutable audit logs',
      'Priority support, named contact',
      'Custom data-residency on request',
    ],
  },
]

export function PricingTable() {
  return (
    <section className="bg-[var(--color-bg)] py-20 sm:py-32">
      <Container>
        <div className="mx-auto max-w-3xl text-center">
          <h1 className="font-display text-4xl tracking-tight text-[var(--color-ink)] sm:text-5xl">
            Pricing
          </h1>
          <p className="mt-4 text-lg tracking-tight text-[var(--color-body)]">
            Self-host for free. Pay us only if you want us to run it for you.
          </p>
        </div>
        <div className="mx-auto mt-16 grid max-w-7xl grid-cols-1 gap-8 lg:grid-cols-3">
          {plans.map((p) => (
            <div
              key={p.name}
              className={
                p.highlight
                  ? 'rounded-3xl bg-[var(--color-ink)] p-10 text-white ring-1 ring-[var(--color-ink)]'
                  : 'rounded-3xl bg-white p-10 text-[var(--color-ink)] ring-1 ring-[var(--color-border)]'
              }
            >
              <h3 className="font-display text-xl">{p.name}</h3>
              <p className={p.highlight ? 'mt-2 text-white/70' : 'mt-2 text-[var(--color-body)]'}>
                {p.description}
              </p>
              <p className="mt-6 font-display text-4xl">
                {p.price}{' '}
                <span className={p.highlight ? 'text-base text-white/70' : 'text-base text-[var(--color-body)]'}>
                  {p.cadence}
                </span>
              </p>
              <Button
                href={p.cta.href}
                variant={p.cta.variant}
                color={p.cta.color}
                className="mt-8 w-full"
              >
                {p.cta.label}
              </Button>
              <ul className={p.highlight ? 'mt-8 space-y-3 text-sm text-white/80' : 'mt-8 space-y-3 text-sm text-[var(--color-body)]'}>
                {p.features.map((f) => (
                  <li key={f} className="flex gap-x-3">
                    <span aria-hidden="true">✓</span>
                    <span>{f}</span>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>
        <p className="mt-12 text-center text-sm text-[var(--color-body)]">
          Cloud pricing in INR; payment via Hyperswitch (UPI / RuPay / Razorpay). Contact us for non-INR billing.
        </p>
      </Container>
    </section>
  )
}
