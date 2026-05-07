import { Container } from '@/components/Container'

const faqs = [
  [
    {
      q: 'Is Raven really free to self-host?',
      a: 'Yes. No telemetry, no upsell wall. The source is public and you keep it.',
    },
    {
      q: 'Can it run on a Raspberry Pi?',
      a: 'Yes. Edge deployment is a first-class target — Raven ships a Compose variant tuned for ARM64 + low memory.',
    },
  ],
  [
    {
      q: 'What is the difference between Self-Hosted and Cloud?',
      a: 'Identical software. Cloud is run by us with SLA, SSO, and audit logs in the region of your choice.',
    },
    {
      q: 'Where does my data go?',
      a: "Self-hosted: nowhere it didn't already go. Cloud: only to our infrastructure, in the region you pick.",
    },
  ],
  [
    {
      q: 'Which models does it support?',
      a: 'Anything OpenAI-API-compatible — Ollama, OpenAI, Anthropic, Groq, vLLM. Embeddings via pgvector with BM25 hybrid search.',
    },
    {
      q: 'How is this different from a hosted Notion AI or a wrapper?',
      a: 'You own the data, the schema, and the keys. Raven is the platform; the surface is yours to extend.',
    },
  ],
]

export function Faqs() {
  return (
    <section
      id="faq"
      aria-label="Frequently asked questions"
      className="bg-[var(--color-bg)] py-20 sm:py-32"
    >
      <Container className="relative">
        <div className="mx-auto max-w-2xl text-center">
          <h2 className="font-display text-3xl tracking-tight text-[var(--color-ink)] sm:text-4xl">
            Frequently asked questions
          </h2>
        </div>
        <ul className="mx-auto mt-16 grid max-w-2xl grid-cols-1 gap-8 lg:max-w-7xl lg:grid-cols-3">
          {faqs.map((column, i) => (
            <li key={i}>
              <ul className="flex flex-col gap-y-8">
                {column.map((faq) => (
                  <li key={faq.q}>
                    <h3 className="font-display text-lg text-[var(--color-ink)]">{faq.q}</h3>
                    <p className="mt-4 text-sm text-[var(--color-body)]">{faq.a}</p>
                  </li>
                ))}
              </ul>
            </li>
          ))}
        </ul>
      </Container>
    </section>
  )
}
