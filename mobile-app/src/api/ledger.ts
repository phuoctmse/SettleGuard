import { env } from '../config/env';
import { fetchJson } from './http';
import type { LedgerEntry } from './types';

export function listEntriesForAccount(accountId: string): Promise<LedgerEntry[]> {
  const url = `${env.ledgerApiUrl}/entries?account_id=${encodeURIComponent(accountId)}`;
  return fetchJson<LedgerEntry[]>(url);
}
