import { env } from '../config/env';
import { fetchJson } from './http';

export interface Notification {
  id: string;
  type: 'risk_hold' | 'settlement_finalized';
  subject_id: string;
  payload: Record<string, unknown>;
  created_at: string;
}

export function listNotifications(): Promise<Notification[]> {
  return fetchJson<Notification[]>(`${env.notificationApiUrl}/notifications`);
}
