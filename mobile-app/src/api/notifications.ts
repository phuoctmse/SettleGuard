import { env } from '../config/env';
import { fetchJson } from './http';
import type { Notification } from './types';

export function listNotifications(): Promise<Notification[]> {
  return fetchJson<Notification[]>(`${env.notificationApiUrl}/notifications`);
}
