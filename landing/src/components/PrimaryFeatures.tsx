import { Container } from '@/components/Container'

const features = [
  {
    title: 'Self-hostable, by design.',
    body:
      'Run on your own server, your own VPC, or a Raspberry Pi at the edge. Your data never leaves your network. No upsell wall, no hidden telemetry.',
  },
  {
    title: 'Multi-tenant from day one.',
    body:
      'Built for teams: workspaces, role-based access, audit trails. SOC 2 and GDPR alignment baked into the schema, not bolted on later.',
  },
  {
    title: 'AI that fits your stack.',
    body:
      'Bring your own models — Ollama, OpenAI, Anthropic, Groq, vLLM, anything OpenAI-API-compatible. pgvector + BM25 hybrid search out of the box.',
  },
]

export function PrimaryFeatures() {
  return (
    <section
      id="features"
      aria-label="Primary features of Raven"
      className="bg-[var(--color-ink)] py-20 text-white sm:py-32"
    >
      <Container>
        <div className="mx-auto max-w-2xl text-center md:mx-0 md:text-left">
          <h2 className="font-display text-3xl tracking-tight text-white sm:text-4xl md:text-5xl">
            Built for teams who own their infrastructure.
          </h2>
          <p className="mt-6 text-lg tracking-tight text-white/70">
            Three properties Raven holds to that most hosted RAG products don&apos;t.
          </p>
        </div>
        <ul className="mt-16 grid grid-cols-1 gap-x-8 gap-y-12 md:grid-cols-3">
          {features.map((f) => (
            <li key={f.title}>
              <h3 className="font-display text-xl font-medium text-white">{f.title}</h3>
              <p className="mt-4 text-base text-white/70">{f.body}</p>
            </li>
          ))}
        </ul>
      </Container>
    </section>
  )
}
