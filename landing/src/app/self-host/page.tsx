import { type Metadata } from 'next'

import { Container } from '@/components/Container'
import { Footer } from '@/components/Footer'
import { Header } from '@/components/Header'
import { CommunitySupport } from '@/components/CommunitySupport'
import { QuickStart } from '@/components/QuickStart'
import { SystemRequirements } from '@/components/SystemRequirements'
import { UpgradeNotes } from '@/components/UpgradeNotes'

export const metadata: Metadata = {
  title: 'Self-host',
  description:
    'Run Raven on your own infrastructure in five minutes. System requirements, one-command Docker Compose, upgrade notes, and community support.',
  alternates: { canonical: 'https://raven.ravencloak.org/self-host/' },
}

export default function SelfHostPage() {
  return (
    <>
      <Header />
      <main>
        <Container className="pt-20 pb-10 text-center lg:pt-28">
          <h1 className="mx-auto max-w-3xl font-display text-4xl tracking-tight text-[var(--color-ink)] sm:text-5xl">
            Self-host Raven on your own infrastructure.
          </h1>
          <p className="mx-auto mt-6 max-w-2xl text-lg text-[var(--color-body)]">
            One Docker Compose command. No telemetry, no callback to a vendor. Runs on
            anything from a 4-GB VPS to a Raspberry Pi 5.
          </p>
        </Container>
        <SystemRequirements />
        <QuickStart />
        <UpgradeNotes />
        <CommunitySupport />
      </main>
      <Footer />
    </>
  )
}
