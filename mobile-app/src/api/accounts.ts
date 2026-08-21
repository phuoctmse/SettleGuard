import { env } from '../config/env';
import { fetchJson } from './http';
import type { Account } from './types';

export function listAccounts(clientId: string): Promise<Account[]> {
  const url = `${env.accountsApiUrl}/accounts?client_id=${encodeURIComponent(clientId)}`;
  return fetchJson<Account[]>(url);
}

export function getAccount(id: string): Promise<Account> {
  return fetchJson<Account>(`${env.accountsApiUrl}/accounts/${id}`);
}
