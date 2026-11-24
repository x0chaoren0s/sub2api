import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import AccountActionMenu from '../AccountActionMenu.vue'
import type { Account } from '@/types'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

function makeAccount(overrides: Partial<Account> = {}): Account {
  return {
    id: 1,
    name: 'test-account',
    platform: 'openai',
    type: 'apikey',
    proxy_id: null,
    concurrency: 3,
    priority: 50,
    status: 'active',
    error_message: null,
    last_used_at: null,
    expires_at: null,
    auto_pause_on_expired: false,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    schedulable: true,
    rate_limited_at: null,
    rate_limit_reset_at: null,
    overload_until: null,
    temp_unschedulable_until: null,
    temp_unschedulable_reason: null,
    session_window_start: null,
    session_window_end: null,
    session_window_status: null,
    ...overrides,
  }
}

const anchorRect = new DOMRect(100, 100, 24, 24)
const getBodyButtons = () => Array.from(document.body.querySelectorAll('button'))

describe('AccountActionMenu — 查看模型能力', () => {
  it('显示菜单项并在点击时发出 models 事件', async () => {
    const account = makeAccount()
    const wrapper = mount(AccountActionMenu, {
      props: { show: true, account, anchorRect },
      attachTo: document.body,
    })

    const button = getBodyButtons().find((el) =>
      el.textContent?.includes('admin.accounts.modelCapabilities.menu')
    )
    expect(button).toBeTruthy()
    await button!.click()
    expect(wrapper.emitted('models')?.[0]?.[0]).toEqual(account)
    expect(wrapper.emitted('close')).toBeTruthy()
    wrapper.unmount()
  })
})
