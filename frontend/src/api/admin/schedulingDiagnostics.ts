import { apiClient } from '../client'

export interface SchedulingDiagnosticAccount {
  id: number
  name: string
  reason?: string
  rate: number | null
  rate_source?: string
  score: number | null
  parts?: Record<string, number>
  ttft_ms: number | null
  error_rate: number
  latency_freshness: number
  order: number
}

export interface SchedulingDiagnostic {
  id: number
  at: string
  group_id: number
  model: string
  layer: string
  selected_id: number
  top_k: number
  cost_enabled: boolean
  sticky_weighted: boolean
  require_compact: boolean
  truncated: boolean
  accounts: SchedulingDiagnosticAccount[]
}

export async function getSchedulingDiagnostics(group?: number): Promise<SchedulingDiagnostic[]> {
  const { data } = await apiClient.get<SchedulingDiagnostic[]>('/admin/accounts/scheduling-diagnostics', {
    params: group ? { group_id: group } : undefined
  })
  return data
}
