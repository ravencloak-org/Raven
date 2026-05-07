import { Container } from '@/components/Container'

const items = [
  {
    q: 'Can I move from Cloud to Self-Hosted?',
    a: 'Yes. We provide a one-shot export tool that reproduces your workspaces, documents, embeddings, and audit history on a fresh self-hosted instance.',
  },
  {
    q: 'Can I move from Self-Hosted to Cloud?',
    a: 'Yes. The same tool runs in reverse.',
  },
  {
    q: 'How are seats counted?',
    a: 'A seat is one human user with sign-in access. Service accounts and read-only API consumers are not seats.',
  },
  {
    q: 'Do you offer a non-profit or open-source discount?',
    a: 'Yes. Email hello@ravencloak.org with a short note about your project.',
  },
]

export function PricingFaqs() {
  return (
    <section className="bg-white py-20 sm:py-32">
      <Container>
        <div className="mx-auto max-w-2xl">
          <h2 className="font-display text-3xl tracking-tight text-[var(--color-ink)] sm:text-4xl">
            Pricing FAQs
          </h2>
          <dl className="mt-12 space-y-8">
            {items.map((it) => (
              <div key={it.q}>
                <dt className="font-display text-lg text-[var(--color-ink)]">{it.q}</dt>
                <dd className="mt-2 text-[var(--color-body)]">{it.a}</dd>
              </div>
            ))}
          </dl>
        </div>
      </Container>
    </section>
  )
}
