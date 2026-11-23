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
})
