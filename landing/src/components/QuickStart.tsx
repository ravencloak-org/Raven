import { Container } from '@/components/Container'

const compose = `# 1. Grab the production Compose file
curl -fsSL https://raven.ravencloak.org/compose.yml -o docker-compose.yml

# 2. Generate secrets and start
docker compose up -d

# 3. Open the app
open http://localhost:8080
`

export function QuickStart() {
  return (
    <section className="bg-[var(--color-bg)] py-20 sm:py-28">
      <Container>
        <div className="mx-auto max-w-3xl">
          <h2 className="font-display text-3xl tracking-tight text-[var(--color-ink)]">
            Five-minute quick start
          </h2>
          <p className="mt-4 text-lg text-[var(--color-body)]">
            One command brings up Raven, PostgreSQL with pgvector, Valkey, and the AI worker.
            Out of the box it points at Ollama on the host; swap in OpenAI/Anthropic by editing one env file.
          </p>
          <pre className="mt-8 overflow-x-auto rounded-2xl bg-[var(--color-ink)] p-6 font-mono text-sm leading-relaxed text-[var(--color-bg)]">
            <code>{compose}</code>
          </pre>
          <p className="mt-6 text-sm text-[var(--color-body)]">
            Full guide and edge / Raspberry Pi variants:{' '}
            <a className="text-[var(--color-accent)] underline" href="https://github.com/ravencloak-org/Raven#self-hosting">
              github.com/ravencloak-org/Raven
            </a>.
          </p>
        </div>
      </Container>
    </section>
  )
}
