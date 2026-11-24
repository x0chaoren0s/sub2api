import type { UpstreamModelMetadata } from '@/api/admin/accounts'

export interface UpstreamModelMetadataSnapshot {
  source?: string
  synced_at?: string
  models?: Record<string, UpstreamModelMetadata>
}

export interface AccountModelCapabilityRow {
  id: string
  displayName?: string
  upstreamId?: string
  reasoning?: boolean
  defaultReasoningLevel?: string
  reasoningLevels: string[]
  inputModalities: string[]
  contextWindow?: number
  maxContextWindow?: number
  maxInputTokens?: number
  maxOutputTokens?: number
  hasMetadata: boolean
}

function asRecord(value: unknown): Record<string, unknown> | null {
  return value && typeof value === 'object' && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null
}

function asString(value: unknown): string | undefined {
  return typeof value === 'string' && value.trim() ? value.trim() : undefined
}

function asNumber(value: unknown): number | undefined {
  return typeof value === 'number' && Number.isFinite(value) && value > 0 ? value : undefined
}

function asBoolean(value: unknown): boolean | undefined {
  return typeof value === 'boolean' ? value : undefined
}

function asStringList(value: unknown): string[] {
  if (!Array.isArray(value)) return []
  return value
    .map((item) => (typeof item === 'string' ? item.trim() : ''))
    .filter(Boolean)
}

export function parseUpstreamModelMetadataSnapshot(extra: unknown): UpstreamModelMetadataSnapshot | null {
  const record = asRecord(extra)
  const raw = record?.upstream_model_metadata
  const snapshot = asRecord(raw)
  if (!snapshot) return null
  const modelsRaw = asRecord(snapshot.models)
  if (!modelsRaw) return null

  const models: Record<string, UpstreamModelMetadata> = {}
  for (const [id, entry] of Object.entries(modelsRaw)) {
    const model = asRecord(entry)
    if (!model) continue
    models[id] = {
      id: asString(model.id) || id,
      display_name: asString(model.display_name),
      description: asString(model.description),
      reasoning: asBoolean(model.reasoning),
      default_reasoning_level: asString(model.default_reasoning_level),
      supported_reasoning_levels: asStringList(model.supported_reasoning_levels),
      input_modalities: asStringList(model.input_modalities),
      context_window: asNumber(model.context_window),
      max_context_window: asNumber(model.max_context_window),
      max_input_tokens: asNumber(model.max_input_tokens),
      max_output_tokens: asNumber(model.max_output_tokens),
    }
  }
  if (Object.keys(models).length === 0) return null
  return {
    source: asString(snapshot.source),
    synced_at: asString(snapshot.synced_at),
    models,
  }
}

export function parseModelMapping(credentials: unknown): Record<string, string> {
  const record = asRecord(credentials)
  const mapping = asRecord(record?.model_mapping)
  if (!mapping) return {}
  const out: Record<string, string> = {}
  for (const [from, to] of Object.entries(mapping)) {
    const publicId = from.trim()
    const upstreamId = asString(to)
    if (!publicId || !upstreamId) continue
    out[publicId] = upstreamId
  }
  return out
}

function rowFromMetadata(
  id: string,
  metadata: UpstreamModelMetadata | undefined,
  upstreamId?: string
): AccountModelCapabilityRow {
  const hasMetadata = Boolean(
    metadata &&
      (metadata.reasoning !== undefined ||
        (metadata.supported_reasoning_levels?.length ?? 0) > 0 ||
        (metadata.input_modalities?.length ?? 0) > 0 ||
        (metadata.context_window ?? 0) > 0 ||
        (metadata.max_context_window ?? 0) > 0 ||
        (metadata.max_input_tokens ?? 0) > 0 ||
        (metadata.max_output_tokens ?? 0) > 0 ||
        Boolean(metadata.display_name))
  )
  return {
    id,
    displayName: metadata?.display_name,
    upstreamId: upstreamId && upstreamId !== id ? upstreamId : undefined,
    reasoning: metadata?.reasoning,
    defaultReasoningLevel: metadata?.default_reasoning_level,
    reasoningLevels: metadata?.supported_reasoning_levels ?? [],
    inputModalities: metadata?.input_modalities ?? [],
    contextWindow: metadata?.context_window,
    maxContextWindow: metadata?.max_context_window,
    maxInputTokens: metadata?.max_input_tokens,
    maxOutputTokens: metadata?.max_output_tokens,
    hasMetadata,
  }
}

function lookupMetadata(
  models: Record<string, UpstreamModelMetadata> | undefined,
  ...ids: Array<string | undefined>
): UpstreamModelMetadata | undefined {
  if (!models) return undefined
  for (const id of ids) {
    if (!id) continue
    if (models[id]) return models[id]
  }
  return undefined
}

/** Build a stable, sorted model+capability list from account credentials/extra. */
export function buildAccountModelCapabilityRows(account: {
  credentials?: Record<string, unknown> | null
  extra?: Record<string, unknown> | null
}): AccountModelCapabilityRow[] {
  const mapping = parseModelMapping(account.credentials)
  const snapshot = parseUpstreamModelMetadataSnapshot(account.extra)
  const models = snapshot?.models
  const rows = new Map<string, AccountModelCapabilityRow>()

  for (const [publicId, upstreamId] of Object.entries(mapping)) {
    rows.set(
      publicId,
      rowFromMetadata(publicId, lookupMetadata(models, upstreamId, publicId), upstreamId)
    )
  }

  if (models) {
    for (const [modelId, metadata] of Object.entries(models)) {
      if (rows.has(modelId)) continue
      const alreadyCovered = [...rows.values()].some((row) => row.upstreamId === modelId)
      if (alreadyCovered) continue
      rows.set(modelId, rowFromMetadata(modelId, metadata))
    }
  }

  return [...rows.values()].sort((a, b) => a.id.localeCompare(b.id))
}
