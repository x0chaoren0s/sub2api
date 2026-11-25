<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.modelCapabilities.title')"
    width="extra-wide"
    @close="emit('close')"
  >
    <div class="space-y-4">
      <div
        v-if="account"
        class="flex flex-wrap items-start justify-between gap-3 rounded-xl border border-primary-200 bg-gradient-to-r from-primary-50 to-primary-100 p-3 dark:border-primary-700/50 dark:from-primary-900/20 dark:to-primary-800/20"
      >
        <div class="min-w-0">
          <div class="truncate font-semibold text-gray-900 dark:text-gray-100">{{ account.name }}</div>
          <div class="mt-1 flex flex-wrap gap-2 text-xs text-gray-500 dark:text-gray-400">
            <span>{{ account.platform }} / {{ account.type }}</span>
            <span v-if="snapshotMeta.source">
              {{ t('admin.accounts.modelCapabilities.source', { source: snapshotMeta.source }) }}
            </span>
            <span v-if="snapshotMeta.syncedAt">
              {{ t('admin.accounts.modelCapabilities.syncedAt', { time: formatSyncedAt(snapshotMeta.syncedAt) }) }}
            </span>
          </div>
        </div>
        <button
          type="button"
          class="inline-flex items-center gap-1.5 rounded-lg bg-white px-3 py-1.5 text-sm font-medium text-primary-700 shadow-sm ring-1 ring-primary-200 transition hover:bg-primary-50 disabled:cursor-not-allowed disabled:opacity-60 dark:bg-dark-800 dark:text-primary-300 dark:ring-primary-800 dark:hover:bg-dark-700"
          :disabled="loading || syncing"
          @click="syncCapabilities"
        >
          <Icon name="sync" size="sm" :class="syncing ? 'animate-spin' : ''" />
          {{ syncing ? t('admin.accounts.syncUpstreamModelsLoading') : t('admin.accounts.modelCapabilities.resync') }}
        </button>
      </div>

      <div v-if="loading" class="flex items-center justify-center py-12">
        <LoadingSpinner />
      </div>

      <div
        v-else-if="error"
        class="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-800/40 dark:bg-red-900/20 dark:text-red-300"
      >
        {{ error }}
      </div>

      <div
        v-else-if="rows.length === 0"
        class="rounded-xl border border-dashed border-gray-200 px-4 py-10 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400"
      >
        {{ t('admin.accounts.modelCapabilities.empty') }}
      </div>

      <div v-else class="overflow-hidden rounded-xl border border-gray-200 dark:border-dark-600">
        <div class="max-h-[60vh] overflow-auto">
          <table class="min-w-full divide-y divide-gray-200 text-sm dark:divide-dark-600">
            <thead class="sticky top-0 bg-gray-50 dark:bg-dark-800">
              <tr class="text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                <th class="px-4 py-3">{{ t('admin.accounts.modelCapabilities.model') }}</th>
                <th class="whitespace-nowrap px-4 py-3">{{ t('admin.accounts.modelCapabilities.multiplier') }}</th>
                <th class="whitespace-nowrap px-4 py-3">{{ t('admin.accounts.modelCapabilities.reasoningLevels') }}</th>
                <th class="whitespace-nowrap px-4 py-3">{{ t('admin.accounts.modelCapabilities.inputTypes') }}</th>
                <th class="whitespace-nowrap px-4 py-3">{{ t('admin.accounts.modelCapabilities.context') }}</th>
                <th class="whitespace-nowrap px-4 py-3">{{ t('admin.accounts.modelCapabilities.outputLimit') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 bg-white dark:divide-dark-700 dark:bg-dark-900">
              <tr v-for="row in rows" :key="row.id" class="align-top">
                <td class="px-4 py-3">
                  <div class="font-medium text-gray-900 dark:text-gray-100">{{ row.id }}</div>
                  <div v-if="row.displayName && row.displayName !== row.id" class="mt-0.5 text-xs text-gray-500">
                    {{ row.displayName }}
                  </div>
                  <div v-if="row.upstreamId" class="mt-0.5 text-xs text-gray-400">
                    {{ t('admin.accounts.modelCapabilities.mapsTo', { model: row.upstreamId }) }}
                  </div>
                </td>
                <td class="whitespace-nowrap px-4 py-3">
                  <div class="flex flex-col items-start gap-1.5">
                    <span v-if="row.creditMultiplier" class="font-medium tabular-nums text-gray-900 dark:text-gray-100">
                      {{ row.creditMultiplier }}
                    </span>
                    <span v-else class="text-xs text-gray-500 dark:text-gray-400">
                      {{ t('admin.accounts.modelCapabilities.undeclared') }}
                    </span>
                    <span
                      v-for="label in row.pricingLabels"
                      :key="label"
                      class="rounded-md bg-teal-50 px-2 py-0.5 text-xs text-teal-700 dark:bg-teal-500/10 dark:text-teal-300"
                    >
                      {{ label }}
                    </span>
                  </div>
                </td>
                <td class="px-4 py-3">
                  <div class="flex flex-wrap gap-1.5">
                    <span v-if="row.reasoning === false" class="text-xs text-gray-500 dark:text-gray-400">
                      {{ t('admin.accounts.modelCapabilities.reasoningNo') }}
                    </span>
                    <template v-else-if="row.reasoningLevels.length || row.defaultReasoningLevel">
                      <span
                        v-for="level in row.reasoningLevels.length ? row.reasoningLevels : [row.defaultReasoningLevel]"
                        :key="`${row.id}-lvl-${level}`"
                        class="rounded-md bg-violet-50 px-2 py-0.5 text-xs text-violet-700 dark:bg-violet-500/10 dark:text-violet-300"
                        :title="level === row.defaultReasoningLevel
                          ? t('admin.accounts.modelCapabilities.defaultReasoning', { level })
                          : undefined"
                      >
                        <strong v-if="level === row.defaultReasoningLevel" class="font-bold">{{ level }}</strong>
                        <template v-else>{{ level }}</template>
                      </span>
                    </template>
                    <span v-else class="text-xs text-gray-500 dark:text-gray-400">
                      {{ row.reasoning
                        ? t('admin.accounts.modelCapabilities.reasoningYes')
                        : t('admin.accounts.modelCapabilities.undeclared') }}
                    </span>
                  </div>
                </td>
                <td class="px-4 py-3">
                  <div class="flex flex-wrap gap-1.5">
                    <span
                      v-for="modality in row.inputModalities"
                      :key="`${row.id}-mod-${modality}`"
                      class="rounded-md bg-sky-100 px-2 py-0.5 text-xs text-sky-700 dark:bg-sky-500/20 dark:text-sky-300"
                    >
                      {{ modalityLabel(modality) }}
                    </span>
                    <span v-if="!row.inputModalities.length" class="text-xs text-gray-500 dark:text-gray-400">
                      {{ t('admin.accounts.modelCapabilities.undeclared') }}
                    </span>
                  </div>
                </td>
                <td class="whitespace-nowrap px-4 py-3 tabular-nums">
                  <div class="flex flex-col items-start gap-1.5">
                    <span
                      v-if="row.contextWindow"
                      class="rounded-md bg-emerald-100 px-2 py-0.5 text-xs text-emerald-700 dark:bg-emerald-500/20 dark:text-emerald-300"
                    >
                      {{ formatTokens(row.contextWindow) }}
                    </span>
                    <span
                      v-if="row.maxContextWindow && row.maxContextWindow !== row.contextWindow"
                      class="rounded-md bg-emerald-50 px-2 py-0.5 text-xs text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-300"
                    >
                      {{ t('admin.accounts.modelCapabilities.maxContextWindow', { value: formatTokens(row.maxContextWindow) }) }}
                    </span>
                    <span
                      v-if="row.maxInputTokens"
                      class="rounded-md bg-teal-100 px-2 py-0.5 text-xs text-teal-700 dark:bg-teal-500/20 dark:text-teal-300"
                    >
                      {{ t('admin.accounts.modelCapabilities.maxInput', { value: formatTokens(row.maxInputTokens) }) }}
                    </span>
                    <span
                      v-if="!row.contextWindow && !row.maxContextWindow && !row.maxInputTokens"
                      class="text-xs text-gray-500 dark:text-gray-400"
                    >
                      {{ t('admin.accounts.modelCapabilities.undeclared') }}
                    </span>
                  </div>
                </td>
                <td class="whitespace-nowrap px-4 py-3 tabular-nums">
                  <span
                    v-if="row.maxOutputTokens"
                    class="inline-block rounded-md bg-amber-100 px-2 py-0.5 text-xs text-amber-700 dark:bg-amber-500/20 dark:text-amber-300"
                  >
                    {{ formatTokens(row.maxOutputTokens) }}
                  </span>
                  <span v-else class="text-xs text-gray-500 dark:text-gray-400">
                    {{ t('admin.accounts.modelCapabilities.undeclared') }}
                  </span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="flex items-center justify-between gap-3">
        <div class="text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.accounts.modelCapabilities.count', { count: rows.length }) }}
        </div>
        <button type="button" class="btn btn-secondary" @click="emit('close')">
          {{ t('common.close') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { Icon } from '@/components/icons'
import { adminAPI } from '@/api/admin'
import type { Account } from '@/types'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatCompactNumber, formatDateTime } from '@/utils/format'
import {
  buildAccountModelCapabilityRows,
  parseUpstreamModelMetadataSnapshot,
  type AccountModelCapabilityRow
} from '@/utils/accountModelCapabilities'

const props = defineProps<{
  show: boolean
  account: Account | null
}>()

const emit = defineEmits<{
  close: []
  updated: [account: Account]
}>()

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const syncing = ref(false)
const error = ref('')
const detail = ref<Account | null>(null)
const rows = ref<AccountModelCapabilityRow[]>([])

const snapshotMeta = computed(() => {
  const snapshot = parseUpstreamModelMetadataSnapshot(detail.value?.extra)
  return {
    source: snapshot?.source || '',
    syncedAt: snapshot?.synced_at || ''
  }
})

const formatTokens = (value: number) => formatCompactNumber(value)
const formatSyncedAt = (value: string) => {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : formatDateTime(date)
}

const modalityLabel = (modality: string) => {
  const key = `admin.accounts.modelCapabilities.modality.${modality}`
  const translated = t(key)
  return translated === key ? modality : translated
}

const applyAccount = (account: Account) => {
  detail.value = account
  rows.value = buildAccountModelCapabilityRows({
    credentials: account.credentials as Record<string, unknown> | undefined,
    extra: account.extra as Record<string, unknown> | undefined
  })
}

const loadDetail = async () => {
  if (!props.account) return
  loading.value = true
  error.value = ''
  try {
    const account = await adminAPI.accounts.getById(props.account.id)
    applyAccount(account)
  } catch (err) {
    error.value = extractApiErrorMessage(err, t('admin.accounts.modelCapabilities.loadFailed'))
    rows.value = []
    detail.value = props.account
  } finally {
    loading.value = false
  }
}

const syncCapabilities = async () => {
  if (!props.account || syncing.value) return
  syncing.value = true
  error.value = ''
  try {
    const result = await adminAPI.accounts.syncUpstreamModels(props.account.id)
    const account = await adminAPI.accounts.getById(props.account.id)
    applyAccount(account)
    emit('updated', account)

    appStore.showSuccess(
      t('admin.accounts.modelCapabilities.resyncSuccess', {
        count: String(result.models?.length ?? rows.value.length)
      })
    )
    if (result.warnings?.some((warning) => warning.code === 'upstream_model_metadata_incomplete')) {
      appStore.showWarning(t('admin.accounts.syncUpstreamModelsMetadataIncomplete'))
    } else if (result.warnings?.some((warning) => warning.code === 'upstream_model_metadata_partial')) {
      appStore.showWarning(t('admin.accounts.syncUpstreamModelsMetadataPartial'))
    }
  } catch (err) {
    error.value = extractApiErrorMessage(err, t('admin.accounts.syncUpstreamModelsFailed'))
    appStore.showError(error.value)
  } finally {
    syncing.value = false
  }
}

watch(
  () => [props.show, props.account?.id] as const,
  ([visible]) => {
    if (!visible || !props.account) {
      if (!visible) {
        detail.value = null
        rows.value = []
        error.value = ''
      }
      return
    }
    applyAccount(props.account)
    void loadDetail()
  },
  { immediate: true }
)
</script>
