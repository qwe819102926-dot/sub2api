import { apiClient } from '../client'

export interface BonusBalanceReport {
  summary: { issued: number; consumed: number; remaining: number; adjustments: number; historical_outflow: number; deleted_user_remaining: number; reconciliation_difference: number }
  sources: { source: string; amount: number; grants: number; users: number }[]
  records: { id: number; user_id: number; email: string; source: string; amount: number; balance_after: number | null; historical: boolean; created_at: string; updated_at: string }[]
  total: number
  page: number
  page_size: number
}

export async function getBonusBalanceReport(params: { page: number; page_size: number; source: string; kind: string; search: string }): Promise<BonusBalanceReport> {
  const { data } = await apiClient.get<BonusBalanceReport>('/admin/bonus-balance', { params })
  return data
}
