import { describe, expect, it } from 'vitest'
import {
  ORBIQO_USAGE_CACHE_TTL_MS,
  isCodebuddyQuotaAccount,
  isLingxiQuotaAccount,
  isOrbiqoQuotaAccount,
  isZcodeQuotaAccount
} from '../zcodeQuota'

describe('zcodeQuota', () => {
  it('exposes a 60s cache TTL aligned with backend orbiqo quota cache', () => {
    expect(ORBIQO_USAGE_CACHE_TTL_MS).toBe(60_000)
  })

  it('detects the three open quota sources used by orbiqo accounts', () => {
    expect(isZcodeQuotaAccount({
      platform: 'anthropic',
      type: 'apikey',
      credentials: { quota_source: 'zcode' }
    })).toBe(true)

    expect(isLingxiQuotaAccount({
      platform: 'openai',
      type: 'apikey',
      credentials: { quota_source: 'lingxi' }
    })).toBe(true)

    expect(isCodebuddyQuotaAccount({
      platform: 'openai',
      type: 'apikey',
      credentials: { quota_source: 'codebuddy' }
    })).toBe(true)

    expect(isOrbiqoQuotaAccount({
      platform: 'openai',
      type: 'oauth',
      credentials: { quota_source: 'lingxi' }
    })).toBe(false)
  })

  it('loads Quark quota for its base URL or an explicit source without guessing shared models', () => {
    for (const credentials of [
      { base_url: 'https://oqk.orbiqo.cc/v1' },
      { quota_source: 'quark', base_url: 'http://127.0.0.1:8022/v1' }
    ]) {
      expect(isOrbiqoQuotaAccount({ platform: 'openai', type: 'apikey', credentials })).toBe(true)
      expect(isOrbiqoQuotaAccount({ platform: 'openai', type: 'oauth', credentials })).toBe(false)
    }
    expect(isOrbiqoQuotaAccount({
      platform: 'openai', type: 'apikey',
      credentials: { base_url: 'https://example.com', model_mapping: { 'qwen3.8-flash': 'qwen3.8-flash' } }
    })).toBe(false)
  })
})
