import { describe, expect, it } from 'vitest'
import {
  buildAccountModelCapabilityRows,
  parseUpstreamModelMetadataSnapshot
} from '../accountModelCapabilities'

describe('accountModelCapabilities', () => {
  it('parses upstream metadata snapshot from account.extra', () => {
    const snapshot = parseUpstreamModelMetadataSnapshot({
      upstream_model_metadata: {
        source: 'upstream',
        synced_at: '2026-11-23T00:00:00Z',
        models: {
          'gpt-5.4': {
            id: 'gpt-5.4',
            reasoning: true,
            supported_reasoning_levels: ['low', 'high'],
            input_modalities: ['text', 'image'],
            context_window: 1_000_000,
            max_input_tokens: 900_000,
            max_output_tokens: 64_000
          }
        }
      }
    })
    expect(snapshot?.source).toBe('upstream')
    expect(snapshot?.models?.['gpt-5.4']?.max_input_tokens).toBe(900_000)
  })

  it('prefers public mapping aliases and resolves metadata via upstream target', () => {
    const rows = buildAccountModelCapabilityRows({
      credentials: {
        model_mapping: {
          'public-alias': 'upstream-model',
          'same-id': 'same-id'
        }
      },
      extra: {
        upstream_model_metadata: {
          models: {
            'upstream-model': {
              id: 'upstream-model',
              display_name: 'Upstream',
              reasoning: true,
              input_modalities: ['text'],
              context_window: 200_000,
              max_output_tokens: 16_000
            },
            'orphan-model': {
              id: 'orphan-model',
              input_modalities: ['text', 'image'],
              context_window: 128_000
            }
          }
        }
      }
    })

    expect(rows.map((row) => row.id)).toEqual(['orphan-model', 'public-alias', 'same-id'])
    expect(rows[1]).toMatchObject({
      id: 'public-alias',
      upstreamId: 'upstream-model',
      reasoning: true,
      contextWindow: 200_000,
      hasMetadata: true
    })
    expect(rows[2].upstreamId).toBeUndefined()
    expect(rows[0].inputModalities).toEqual(['text', 'image'])
  })

  it('marks models without capability fields as incomplete', () => {
    const rows = buildAccountModelCapabilityRows({
      credentials: { model_mapping: { bare: 'bare' } },
      extra: { upstream_model_metadata: { models: {} } }
    })
    expect(rows).toEqual([
      expect.objectContaining({ id: 'bare', hasMetadata: false, reasoningLevels: [], inputModalities: [] })
    ])
  })
})
