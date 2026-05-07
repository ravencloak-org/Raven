import { Button } from '@/components/Button'
import { Container } from '@/components/Container'

const plans = [
  {
    name: 'Self-Hosted',
    headline: 'Free forever. Source-available.',
    price: '₹0',
    cta: { label: 'Self-host in 5 min', href: '/self-host', variant: 'solid' as const },
  },
  {
    name: 'Cloud',
    headline: 'Managed by us, ready to scale.',
    price: 'From ₹X / seat / month',
    cta: { label: 'See pricing', href: '/pricing', variant: 'outline' as const },
  },
]

export function PricingTeaser() {
  return (
    <section
      id="pricing"
      aria-label="Pricing summary"
      className="bg-[var(--color-bg)] py-20 sm:py-32"
    >
      <Container>
        <div className="mx-auto max-w-2xl text-center">
          <h2 className="font-display text-3xl tracking-tight text-[var(--color-ink)] sm:text-4xl">
            Free if you run it. Reasonable if we run it.
          </h2>
          <p className="mt-4 text-lg tracking-tight text-[var(--color-body)]">
            Self-host the whole thing for free, or let us run it for you on managed Cloud with SLA, SSO, and audit logs.
          </p>
        </div>
        <div className="mx-auto mt-16 grid max-w-4xl grid-cols-1 gap-8 md:grid-cols-2">
          {plans.map((p) => (
            <div
              key={p.name}
              className="rounded-3xl bg-white p-8 ring-1 ring-[var(--color-border)]"
            >
              <h3 className="font-display text-xl text-[var(--color-ink)]">{p.name}</h3>
              <p className="mt-2 text-[var(--color-body)]">{p.headline}</p>
              <p className="mt-6 font-display text-3xl text-[var(--color-ink)]">{p.price}</p>
              <Button
                href={p.cta.href}
                variant={p.cta.variant}
                color={p.cta.variant === 'outline' ? 'ink' : 'accent'}
                className="mt-8 w-full"
              >
                {p.cta.label}
              </Button>
            </div>
          ))}
        </div>
      </Container>
    </section>
  )
}
