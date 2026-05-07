import { type Metadata } from 'next'

import { CallToAction } from '@/components/CallToAction'
import { Faqs } from '@/components/Faqs'
import { Footer } from '@/components/Footer'
import { Header } from '@/components/Header'
import { Hero } from '@/components/Hero'
import { PricingTeaser } from '@/components/PricingTeaser'
import { PrimaryFeatures } from '@/components/PrimaryFeatures'
import { SecondaryFeatures } from '@/components/SecondaryFeatures'

export const metadata: Metadata = {
  alternates: { canonical: 'https://raven.ravencloak.org/' },
}

export default function Home() {
  return (
    <>
      <Header />
      <main>
        <Hero />
        <PrimaryFeatures />
        <SecondaryFeatures />
        <CallToAction />
        <PricingTeaser />
        <Faqs />
      </main>
      <Footer />
    </>
  )
}
