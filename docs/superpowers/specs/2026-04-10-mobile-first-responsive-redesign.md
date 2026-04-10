# Mobile-First Responsive Redesign

**Issue**: #200  
**Date**: 2026-04-10  
**Status**: Approved  

## Context

The Vue 3 frontend is built for desktop. The primary WhatsApp use case is mobile-first — operators managing calls on phones. All pages need to be usable on a 390px viewport without horizontal scrolling.

### Current State

- **UI framework**: Tailwind CSS v4.2.2, no component library — all custom
- **Mobile detection**: `useMobile()` composable exists (768px breakpoint) but is underutilized
- **Sidebar**: Already has mobile slide-out mode (w-64 fixed, backdrop overlay)
- **Touch targets**: Most buttons already enforce `min-h-[44px] min-w-[44px]`
- **Tables**: Most list pages use desktop-only `<table>` elements — no card fallback
- **Modals**: Fixed `max-w-md` (448px) — don't adapt to mobile screens
- **E2E tests**: Only Desktop Chrome viewport — no mobile viewport tests

### Breakpoint Strategy

Single breakpoint at `md:` (768px) — matches the existing `useMobile()` composable. Below = mobile layout, above = desktop layout.

---

## Design

### 1. Navigation: Bottom Tab Bar

**Desktop** (≥768px): Keep the existing 64px icon-only sidebar (`w-16`).

**Mobile** (<768px): Replace the slide-out sidebar with a fixed bottom tab bar.

#### Tab mapping (5 tabs)

| Tab | Icon | Route | Priority |
|-----|------|-------|----------|
| Home | house | `/dashboard` | Overview |
| Voice | microphone | `/orgs/:orgId/voice` | Voice sessions |
| Calls | phone | `/orgs/:orgId/whatsapp/calls` | Primary operator flow |
| Numbers | smartphone | `/orgs/:orgId/whatsapp/phone-numbers` | Phone management |
| More | ellipsis-h | bottom sheet | Secondary pages |

#### "More" bottom sheet contents

- Dashboard (if not already on Home)
- Knowledge Bases
- Chatbot Config
- Analytics
- API Keys
- LLM Providers
- Sandbox

#### Implementation

New component: `MobileTabBar.vue`

```vue
<template>
  <!-- Only rendered when isMobile -->
  <nav class="fixed bottom-0 inset-x-0 z-50 bg-slate-800 border-t border-slate-700
              flex justify-around items-center pb-[env(safe-area-inset-bottom)] min-h-[56px]">
    <TabItem v-for="tab in tabs" :key="tab.name" v-bind="tab" />
  </nav>
</template>
```

- Active tab: indigo-500 icon + label color, white icon stroke
- Inactive: slate-400 icon + label
- All touch targets: min 56×44px
- Safe-area padding at bottom for notched phones: `pb-[env(safe-area-inset-bottom)]`
- FAB (floating action button) for "New Call" on the Calls page — positioned above tab bar

#### Changes to existing components

- `DefaultLayout.vue`: Conditionally render `AppSidebar` (desktop) or `MobileTabBar` (mobile)
- `AppSidebar.vue`: Add `v-if="!isMobile"` — no longer needed on mobile
- `AppHeader.vue`: Remove hamburger menu button on mobile (no sidebar to toggle)
- Main content area: Add `pb-20` on mobile to prevent content hiding behind tab bar

#### "More" bottom sheet

New component: `BottomSheet.vue` — reusable slide-up panel.

```ts
Props: { open: boolean, title?: string }
Emits: ['close']
```

- Backdrop: `bg-black/50` with fade transition
- Sheet: slides up from bottom, `rounded-t-2xl`, drag handle at top
- Menu items: 48px min-height, icon + label, full-width tap area

---

### 2. Table → Card Conversions

All list pages conditionally render a `<table>` (desktop) or stacked cards (mobile) based on `useMobile()`.

#### Card design pattern

Every card follows this consistent structure:

```text
┌─────────────────────────────────┐
│ Title                   [Badge] │
│ subtitle • metadata             │
│                                 │
│ secondary info / progress       │
├─────────────────────────────────┤
│ timestamp              [Action] │  ← optional action row
└─────────────────────────────────┘
```

- Background: `bg-slate-800` with `rounded-xl` (12px)
- Padding: 14px
- Title: white, font-semibold, 15px
- Subtitle: slate-400, 12px, bullet-separated
- Badge: pill-shaped, top-right (status colors: green=active, amber=indexing, red=error, slate=inactive)
- Action row: separated by `border-t border-slate-700`, only shown when inline actions exist
- Drill-down cards: chevron `›` icon on the right edge
- Color-coded left border for status-heavy lists (calls): `border-l-3` with green/blue/slate

#### Pages affected

| Page | Desktop | Mobile Card Content |
|------|---------|-------------------|
| `KBListPage.vue` | table | name, model+doc count, status badge, progress bar if indexing |
| `WorkspaceListPage.vue` | table | initial avatar, name, KB+member count, chevron |
| `ApiKeyListPage.vue` | table | name, masked key (mono), status, created date, revoke button |
| `VoiceSessionListPage.vue` | table | session name, state badge, duration, timestamp |
| `CallsPage.vue` | already has cards | refine: color-coded border, LIVE/RING/END badges |
| `PhoneNumbersPage.vue` | already has mobile cards | already done — no changes needed |
| `LlmProviderListPage.vue` | table | provider name, model count, status |

#### Implementation approach

Each page uses conditional rendering:

```vue
<template>
  <!-- Desktop table -->
  <table v-if="!isMobile" class="w-full">...</table>

  <!-- Mobile cards -->
  <div v-else class="flex flex-col gap-2.5">
    <div v-for="item in items" class="bg-slate-800 rounded-xl p-3.5">
      ...card layout...
    </div>
  </div>
</template>
```

No shared `DataCard` component — each page renders its own card markup inline. The pattern is simple enough that a shared component would add indirection without reducing code.

---

### 3. Modals → Full-Screen / Bottom Sheet

#### Form modals (create/edit flows)

Desktop: Centered overlay modal (`max-w-md`, backdrop blur).

Mobile: Full-screen page with:
- Top bar: back arrow (←) left, title center, empty right spacer
- Scrollable form body with larger inputs (min-height 48px, font-size 15px)
- Sticky bottom CTA: full-width primary button with safe-area padding

Affected modals:
- `CreateSessionModal.vue` (voice)
- `InitiateCallModal.vue` (WhatsApp)
- Create API Key dialog (inline in `ApiKeyListPage.vue`)
- Add Phone Number form (inline in `PhoneNumbersPage.vue`)
- Create Workspace form (inline in `WorkspaceListPage.vue`)

#### Confirmation dialogs (destructive actions)

Desktop: Small centered dialog with side-by-side Cancel/Confirm buttons.

Mobile: Bottom sheet with:
- Drag handle at top
- Warning icon + title
- Description text
- Stacked full-width buttons: destructive action on top (red), cancel below (slate)

Affected dialogs:
- Revoke API Key
- Delete Session
- Delete KB

#### Implementation

Wrap existing modals in a responsive container:

```vue
<template>
  <!-- Desktop: centered overlay -->
  <div v-if="!isMobile" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
    <div class="bg-slate-800 rounded-xl max-w-md w-full mx-4">
      <slot />
    </div>
  </div>

  <!-- Mobile: full-screen -->
  <div v-else class="fixed inset-0 z-50 bg-slate-900 flex flex-col">
    <header class="flex items-center h-14 px-4 border-b border-slate-700">
      <button @click="$emit('close')" class="min-h-[44px] min-w-[44px]">←</button>
      <span class="flex-1 text-center font-semibold">{{ title }}</span>
      <div class="w-11" />
    </header>
    <div class="flex-1 overflow-y-auto p-4">
      <slot />
    </div>
    <div class="p-4 pb-7 border-t border-slate-700">
      <slot name="actions" />
    </div>
  </div>
</template>
```

New shared component: `ResponsiveModal.vue` — wraps any modal content and handles desktop/mobile rendering.

---

### 4. Form Adaptations

Forms already use `w-full` inputs and `space-y-4` vertical stacking — they work on mobile. Minor adjustments:

- Input min-height: `min-h-[48px]` on mobile (vs default ~40px)
- Font size: `text-[15px]` on mobile inputs for readability and to prevent iOS zoom
- Labels: keep `text-sm` but add `font-medium` for contrast
- Button groups: stack vertically on mobile (`flex-col` vs `flex-row`)
- `ChatbotConfiguratorPage.vue` (434 lines, largest form): sections already stack naturally — just ensure no fixed widths

---

### 5. Playwright Mobile Viewport Tests

Add mobile viewport project to `playwright.config.ts`:

```typescript
projects: [
  { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
  { name: 'mobile-chrome', use: { ...devices['Pixel 7'] } },
]
```

Add 3 mobile E2E tests for critical flows:

1. **Login flow** (`e2e/mobile/login.spec.ts`): navigate to login, verify no horizontal scroll, verify touch targets
2. **Navigation** (`e2e/mobile/navigation.spec.ts`): verify bottom tab bar renders, tap each tab, open "More" sheet
3. **Call list** (`e2e/mobile/calls.spec.ts`): verify cards render (not table), tap a call card, verify detail page

---

## Component Inventory

### New components

| Component | Purpose |
|-----------|---------|
| `MobileTabBar.vue` | Bottom tab bar navigation (mobile only) |
| `BottomSheet.vue` | Reusable slide-up panel for "More" menu and confirm dialogs |
| `ResponsiveModal.vue` | Wrapper that renders centered modal (desktop) or full-screen (mobile) |

### Modified components

| Component | Change |
|-----------|--------|
| `DefaultLayout.vue` | Conditional sidebar vs tab bar, mobile bottom padding |
| `AppSidebar.vue` | Hide on mobile (`v-if="!isMobile"`) |
| `AppHeader.vue` | Remove hamburger on mobile |
| `KBListPage.vue` | Add mobile card view |
| `WorkspaceListPage.vue` | Add mobile card view |
| `ApiKeyListPage.vue` | Add mobile card view + bottom sheet confirm |
| `VoiceSessionListPage.vue` | Add mobile card view |
| `CallsPage.vue` | Refine existing card layout |
| `LlmProviderListPage.vue` | Add mobile card view |
| `CreateSessionModal.vue` | Wrap in ResponsiveModal |
| `InitiateCallModal.vue` | Wrap in ResponsiveModal |
| `playwright.config.ts` | Add mobile-chrome project |

### Unchanged

- `AuthLayout.vue` — already responsive (max-w-md centered)
- `DashboardPage.vue` — simple metrics, stacks naturally
- `PhoneNumbersPage.vue` — already has mobile card view
- `ChatbotConfiguratorPage.vue` — forms stack naturally, no table
- `AnalyticsDashboardPage.vue` — charts, out of scope for this pass
- Legal pages — static content, already readable

---

## Acceptance Criteria

- All pages usable without horizontal scrolling at 390px width
- Bottom tab bar visible on mobile with all 5 tabs functional
- All list pages show cards (not tables) on mobile
- Form modals render full-screen on mobile with sticky CTA
- Confirm dialogs render as bottom sheets on mobile
- Touch targets ≥ 44×44px on all interactive elements
- Playwright mobile viewport tests pass for login, navigation, call list
- Lighthouse mobile score ≥ 80 for dashboard page
