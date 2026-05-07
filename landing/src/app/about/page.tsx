import { type Metadata } from 'next'

import { Footer } from '@/components/Footer'
import { Header } from '@/components/Header'
import { Maintainers } from '@/components/Maintainers'
import { Mission } from '@/components/Mission'

export const metadata: Metadata = {
  title: 'About',
  description:
    'Why Raven exists, and who maintains it. A self-hostable, multi-tenant RAG platform built by Jobin Lawrance.',
  alternates: { canonical: 'https://raven.ravencloak.org/about/' },
}

export default function AboutPage() {
  return (
    <>
      <Header />
      <main>
        <Mission />
        <Maintainers />
      </main>
      <Footer />
    </>
  )
}
