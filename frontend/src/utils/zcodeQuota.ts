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

const LINGXI_MODEL_IDS = new Set(['plan_pro', 'plan_max', 'lingxi-pro', 'lingxi-max'])

/** OpenAI API Key 账号指向 open-lingxi 时，拉取智点余额。 */
export function isLingxiQuotaAccount(account: Pick<Account, 'platform' | 'type' | 'credentials'>): boolean {
  if (account.platform !== 'openai' || account.type !== 'apikey') return false
  const credentials = account.credentials as Record<string, unknown> | undefined
  if (!credentials) return false
  if (credentials.quota_source === 'lingxi') return true
  if (typeof credentials.base_url === 'string' && /(?:^|\/\/)olx\./i.test(credentials.base_url)) return true
  const mapping = credentials.model_mapping
  if (!mapping || typeof mapping !== 'object') return false
  return Object.keys(mapping as Record<string, unknown>).some((key) => LINGXI_MODEL_IDS.has(key))
}

export function isOrbiqoQuotaAccount(account: Pick<Account, 'platform' | 'type' | 'credentials'>): boolean {
  return isZcodeQuotaAccount(account) || isLingxiQuotaAccount(account) || isCodebuddyQuotaAccount(account)
}

const CODEBUDDY_MODEL_IDS = new Set(['hy4free', 'fast-model', 'space-bunny'])

/** OpenAI API Key 账号指向 open-codebuddy-cn 时，拉取 WorkBuddy 积分。 */
export function isCodebuddyQuotaAccount(account: Pick<Account, 'platform' | 'type' | 'credentials'>): boolean {
  if (account.platform !== 'openai' || account.type !== 'apikey') return false
  const credentials = account.credentials as Record<string, unknown> | undefined
  if (!credentials) return false
  if (credentials.quota_source === 'codebuddy') return true
  if (typeof credentials.base_url === 'string' && /(?:^|\/\/)owkbd\./i.test(credentials.base_url)) return true
  const mapping = credentials.model_mapping
  if (!mapping || typeof mapping !== 'object') return false
  return Object.keys(mapping as Record<string, unknown>).some((key) => CODEBUDDY_MODEL_IDS.has(key))
}
