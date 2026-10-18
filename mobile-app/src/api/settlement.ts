import { env } from '../config/env';
import { fetchJson } from './http';
import type { Page, Settlement, Transaction } from './types';

function withCursor(url: string, cursor?: string): string {
  if (!cursor) return url;
  return `${url}${url.includes('?') ? '&' : '?'}cursor=${encodeURIComponent(cursor)}`;
}

export function listHeldTransactions(cursor?: string): Promise<Page<Transaction>> {
  return fetchJson<Page<Transaction>>(
    withCursor(`${env.settlementApiUrl}/transactions?status=held`, cursor),
  );
}

export function approveTransaction(id: string): Promise<void> {
  return fetchJson<void>(`${env.settlementApiUrl}/transactions/${id}/approve`, { method: 'POST' });
}

export function rejectTransaction(id: string): Promise<void> {
  return fetchJson<void>(`${env.settlementApiUrl}/transactions/${id}/reject`, { method: 'POST' });
}

export function listSettlements(cursor?: string): Promise<Page<Settlement>> {
  return fetchJson<Page<Settlement>>(withCursor(`${env.settlementApiUrl}/settlements`, cursor));
}
