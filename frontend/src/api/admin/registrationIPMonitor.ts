import { apiClient } from '../client'
export interface RegistrationIPConfig { enabled: boolean; window_hours: number; max_accounts: number; ban_hours: number }
export interface RegistrationIPRow { ip: string; total: number; recent: number; last_registered_at: string; blocked_at: string | null; blocked_until: string | null; active: boolean; ban_count: number }
export interface RegistrationIPAccount { user_id: number; email: string; source: string; created_at: string; status: string; deleted: boolean }
export interface RegistrationIPReport { config: RegistrationIPConfig; rows: RegistrationIPRow[]; accounts: RegistrationIPAccount[]; total: number; account_total: number; page: number }
export async function getRegistrationIPReport(params: { search: string; ip: string; page: number }): Promise<RegistrationIPReport> {
 const { data } = await apiClient.get<RegistrationIPReport>('/admin/registration-ip-monitor', { params }); return data
}
export async function saveRegistrationIPConfig(config: RegistrationIPConfig): Promise<RegistrationIPConfig> {
 const { data } = await apiClient.put<RegistrationIPConfig>('/admin/registration-ip-monitor/config', config); return data
}
export async function unblockRegistrationIP(ip: string): Promise<void> { await apiClient.post('/admin/registration-ip-monitor/unblock', { ip }) }
