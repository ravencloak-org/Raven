import { Button } from '@/components/Button'
import { Container } from '@/components/Container'

export function CallToAction() {
  return (
    <section
      id="get-started"
      className="relative overflow-hidden bg-[var(--color-ink)] py-32"
    >
      <Container className="relative">
        <div className="mx-auto max-w-lg text-center">
          <h2 className="font-display text-3xl tracking-tight text-white sm:text-4xl">
            Ready to give your team a brain that doesn&apos;t leak?
          </h2>
          <p className="mt-4 text-lg tracking-tight text-white/70">
            Five minutes from <code className="font-mono text-[var(--color-accent)]">docker compose up</code> to a
            working voice + chat search over your team&apos;s documents.
          </p>
          <Button href="/self-host" color="accent" className="mt-10">
            Read the self-host guide
          </Button>
        </div>
      </Container>
    </section>
  )
}
