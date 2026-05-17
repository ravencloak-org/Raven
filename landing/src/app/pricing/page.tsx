import { type Metadata } from 'next'

import { Footer } from '@/components/Footer'
import { Header } from '@/components/Header'
import { PricingFaqs } from '@/components/PricingFaqs'
import { PricingTable } from '@/components/PricingTable'

export const metadata: Metadata = {
  title: 'Pricing',
  description:
    'Raven is free to self-host. Pro from ₹1,700 / seat / month (min 5 seats), Enterprise from ₹3,500 / seat / month (min 20 seats).',
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
