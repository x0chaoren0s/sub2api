import type { Account } from '@/types'

/** Anthropic API Key 账号在显式标记或使用 zcode-provider 短别名时，才去拉额度。 */
export function isZcodeQuotaAccount(account: Pick<Account, 'platform' | 'type' | 'credentials'>): boolean {
  if (account.platform !== 'anthropic' || account.type !== 'apikey') return false
  const credentials = account.credentials as Record<string, unknown> | undefined
  if (!credentials) return false
  if (credentials.quota_source === 'zcode') return true
  const mapping = credentials.model_mapping
  if (!mapping || typeof mapping !== 'object') return false
  return Object.entries(mapping as Record<string, unknown>).some(
    ([key, value]) => isZcodeModelAlias(key) || (typeof value === 'string' && isZcodeModelAlias(value))
  )
}

function isZcodeModelAlias(value: string): boolean {
  return value.startsWith('c/') || value.startsWith('s/')
}
