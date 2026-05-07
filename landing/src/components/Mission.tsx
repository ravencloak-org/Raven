import { Container } from '@/components/Container'

export function Mission() {
  return (
    <section className="bg-white py-20 sm:py-32">
      <Container>
        <div className="mx-auto max-w-3xl">
          <h1 className="font-display text-4xl tracking-tight text-[var(--color-ink)] sm:text-5xl">
            Knowledge platforms shouldn&apos;t be data brokers.
          </h1>
          <p className="mt-6 text-lg text-[var(--color-body)]">
            Raven exists because most &ldquo;AI for your team&rdquo; products are wrappers that
            ship your documents to someone else&apos;s servers, lock you into one model
            vendor, and treat self-hosting as a nice-to-have at the enterprise tier.
            We think it should be the default.
          </p>
          <p className="mt-4 text-lg text-[var(--color-body)]">
            Raven is a self-hostable, multi-tenant RAG platform with first-class
            voice and chat surfaces. It runs on your hardware, with your models,
            under your access controls — and it does that on a Raspberry Pi just as
            well as it does on a fleet of VMs.
          </p>
        </div>
      </Container>
    </section>
  )
}
