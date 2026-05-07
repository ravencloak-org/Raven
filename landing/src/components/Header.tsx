'use client'

import Link from 'next/link'
import { Popover, PopoverBackdrop, PopoverButton, PopoverPanel } from '@headlessui/react'
import clsx from 'clsx'

import { Button } from '@/components/Button'
import { Container } from '@/components/Container'
import { Logo } from '@/components/Logo'
import { NavLink } from '@/components/NavLink'

const REPO_URL = 'https://github.com/ravencloak-org/Raven'

function MobileNavLink({ href, children }: { href: string; children: React.ReactNode }) {
  return (
    <PopoverButton as={Link} href={href} className="block w-full p-2">
      {children}
    </PopoverButton>
  )
}

function MobileNavIcon({ open }: { open: boolean }) {
  return (
    <svg
      aria-hidden="true"
      className="h-3.5 w-3.5 overflow-visible stroke-[var(--color-ink)]"
      fill="none"
      strokeWidth={2}
      strokeLinecap="round"
    >
      <path d="M0 1H14M0 7H14M0 13H14" className={clsx('origin-center transition', open && 'scale-90 opacity-0')} />
      <path d="M2 2L12 12M12 2L2 12" className={clsx('origin-center transition', !open && 'scale-90 opacity-0')} />
    </svg>
  )
}

function MobileNavigation() {
  return (
    <Popover>
      <PopoverButton
        className="relative z-10 flex h-8 w-8 items-center justify-center focus:not-data-focus:outline-hidden"
        aria-label="Toggle navigation"
      >
        {({ open }) => <MobileNavIcon open={open} />}
      </PopoverButton>
      <PopoverBackdrop
        transition
        className="fixed inset-0 bg-[var(--color-ink)]/20 duration-150 data-closed:opacity-0 data-enter:ease-out data-leave:ease-in"
      />
      <PopoverPanel
        transition
        className="absolute inset-x-0 top-full mt-4 flex origin-top flex-col rounded-2xl bg-white p-4 text-lg tracking-tight text-[var(--color-ink)] shadow-xl ring-1 ring-[var(--color-border)] data-closed:scale-95 data-closed:opacity-0 data-enter:duration-150 data-enter:ease-out data-leave:duration-100 data-leave:ease-in"
      >
        <MobileNavLink href="/#features">Features</MobileNavLink>
        <MobileNavLink href="/pricing">Pricing</MobileNavLink>
        <MobileNavLink href="/self-host">Self-host</MobileNavLink>
        <MobileNavLink href="/about">About</MobileNavLink>
        <hr className="my-2 border-[var(--color-border)]" />
        <MobileNavLink href={REPO_URL}>GitHub</MobileNavLink>
      </PopoverPanel>
    </Popover>
  )
}

export function Header() {
  return (
    <header className="py-6">
      <Container>
        <nav className="relative z-50 flex justify-between">
          <div className="flex items-center md:gap-x-12">
            <Link href="/" aria-label="Home">
              <Logo variant="mark" markClassName="h-8 w-auto md:hidden" />
              <Logo variant="full" markClassName="h-8 w-auto" className="hidden md:inline-flex" />
            </Link>
            <div className="hidden md:flex md:gap-x-6">
              <NavLink href="/#features">Features</NavLink>
              <NavLink href="/pricing">Pricing</NavLink>
              <NavLink href="/self-host">Self-host</NavLink>
              <NavLink href="/about">About</NavLink>
            </div>
          </div>
          <div className="flex items-center gap-x-5 md:gap-x-8">
            <Button href={REPO_URL} variant="outline" color="ink" className="hidden md:inline-flex">
              GitHub
            </Button>
            <Button href="/self-host">Self-host in 5 min</Button>
            <div className="-mr-1 md:hidden">
              <MobileNavigation />
            </div>
          </div>
        </nav>
      </Container>
    </header>
  )
}
