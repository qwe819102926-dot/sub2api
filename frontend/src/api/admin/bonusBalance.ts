import { apiClient } from '../client'

export interface BonusBalanceReport {
  scope: 'current' | 'history'
  started_at: string
  summary: { grants: number; users: number; today_issued: number; pending: number; issued: number; consumed: number; remaining: number; adjustments: number; historical_outflow: number; deleted_user_remaining: number; reconciliation_difference: number }
  sources: { source: string; amount: number; grants: number; users: number }[]
  records: { id: number; user_id: number; email: string; source: string; amount: number; balance_after: number | null; historical: boolean; created_at: string; updated_at: string }[]
  total: number
  page: number
  page_size: number
}

export async function getBonusBalanceReport(params: { scope?: 'current' | 'history'; start_date?: string; end_date?: string; page: number; page_size: number; source: string; kind: string; search: string }): Promise<BonusBalanceReport> {
  const { data } = await apiClient.get<BonusBalanceReport>('/admin/bonus-balance', { params })
  return data
}

export interface BonusBalanceConversion {
  completed: boolean
  users: number
  before_total: string
  after_total: string
  completed_at: string | null
}

export async function getBonusBalanceConversion(): Promise<BonusBalanceConversion> {
  const { data } = await apiClient.get<BonusBalanceConversion>('/admin/bonus-balance/conversion')
  return data
}

export async function convertBonusBalances(confirmation: string): Promise<BonusBalanceConversion> {
  const { data } = await apiClient.post<BonusBalanceConversion>('/admin/bonus-balance/conversion', { confirmation }, { timeout: 60000 })
  return data
}
