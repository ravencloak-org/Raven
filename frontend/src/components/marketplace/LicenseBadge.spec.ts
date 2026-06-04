import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import LicenseBadge from './LicenseBadge.vue'

describe('LicenseBadge', () => {
  it('renders friendly label + tooltip for known SPDX ids', () => {
    const wrapper = mount(LicenseBadge, { props: { license: 'CC-BY-SA-4.0' } })
    expect(wrapper.text()).toBe('CC BY-SA 4.0')
    expect(wrapper.attributes('title')).toContain('same license')
  })

  it('falls back to the raw id for unknown SPDX entries', () => {
    const wrapper = mount(LicenseBadge, { props: { license: 'BSD-3-Clause' } })
    expect(wrapper.text()).toBe('BSD-3-Clause')
    expect(wrapper.attributes('title')).toContain('BSD-3-Clause')
  })

  it('renders "Unlicensed" when license is missing', () => {
    const wrapper = mount(LicenseBadge, { props: { license: null } })
    expect(wrapper.text()).toBe('Unlicensed')
  })
})
