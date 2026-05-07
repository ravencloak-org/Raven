'use client'

import { useState } from 'react'
import Image, { type StaticImageData } from 'next/image'
import { Tab, TabGroup, TabList, TabPanel, TabPanels } from '@headlessui/react'
import clsx from 'clsx'

import { Container } from '@/components/Container'
import voiceImg from '@/images/screenshots/voice.svg'
import chatImg from '@/images/screenshots/chat.svg'
import whatsappImg from '@/images/screenshots/whatsapp.svg'

type Feature = { title: string; summary: string; image: StaticImageData | string }

const features: Feature[] = [
  {
    title: 'Voice',
    summary:
      'Conversational search with low-latency LiveKit. Talk to your knowledge base, get cited answers in real time.',
    image: voiceImg,
  },
  {
    title: 'Chat',
    summary:
      'Real-time multi-user chat with citations and source previews. Share threads with your team without losing context.',
    image: chatImg,
  },
  {
    title: 'Channels',
    summary:
      'Ingest from WhatsApp, Slack, email, and the web. Raven keeps the source link so every answer stays traceable.',
    image: whatsappImg,
  },
]

export function SecondaryFeatures() {
  const [tab, setTab] = useState(0)
  return (
    <section
      id="secondary-features"
      aria-label="Surfaces and integrations"
      className="bg-[var(--color-bg)] py-20 sm:py-32"
    >
      <Container>
        <div className="mx-auto max-w-2xl text-center">
          <h2 className="font-display text-3xl tracking-tight text-[var(--color-ink)] sm:text-4xl">
            Three surfaces, one knowledge base.
          </h2>
          <p className="mt-4 text-lg tracking-tight text-[var(--color-body)]">
            Voice, chat, and ingestion channels — share the same indexes, the same access controls, the same audit trail.
          </p>
        </div>
        <TabGroup
          className="mt-16 grid grid-cols-1 items-center gap-y-2 pt-10 sm:gap-y-6 md:mt-20 lg:grid-cols-12 lg:pt-0"
          selectedIndex={tab}
          onChange={setTab}
        >
          <TabList className="-mx-4 flex overflow-x-auto pb-4 sm:mx-0 sm:flex-col sm:overflow-visible sm:pb-0 lg:col-span-5">
            {features.map((f, i) => (
              <Tab
                key={f.title}
                className={({ selected }: { selected: boolean }) =>
                  clsx(
                    'group relative rounded-lg px-4 py-1 text-left ring-1 transition focus:outline-none lg:rounded-l-xl lg:rounded-r-none lg:p-6',
                    selected
                      ? 'bg-white ring-[var(--color-border)]'
                      : 'ring-transparent hover:bg-white/40',
                  )
                }
              >
                <h3 className="font-display text-lg text-[var(--color-ink)]">{f.title}</h3>
                <p className="mt-2 hidden text-sm text-[var(--color-body)] lg:block">{f.summary}</p>
              </Tab>
            ))}
          </TabList>
          <TabPanels className="lg:col-span-7">
            {features.map((f) => (
              <TabPanel key={f.title} className="rounded-2xl bg-white p-4 ring-1 ring-[var(--color-border)]">
                <Image
                  src={f.image}
                  alt={f.title + ' screenshot'}
                  className="w-full"
                  width={760}
                  height={460}
                />
              </TabPanel>
            ))}
          </TabPanels>
        </TabGroup>
      </Container>
    </section>
  )
}
