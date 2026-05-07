import { type Metadata } from 'next'

import { Footer } from '@/components/Footer'
import { Header } from '@/components/Header'
import { PricingFaqs } from '@/components/PricingFaqs'
import { PricingTable } from '@/components/PricingTable'

export const metadata: Metadata = {
  title: 'Pricing',
  description:
    'Raven is free to self-host. Cloud starts from ₹X / seat / month. Compare Self-Hosted, Cloud Starter, and Cloud Pro.',
  alternates: { canonical: 'https://raven.ravencloak.org/pricing/' },
}

export default function PricingPage() {
  return (
    <>
      <Header />
      <main>
        <PricingTable />
        <PricingFaqs />
      </main>
      <Footer />
    </>
  )
}
