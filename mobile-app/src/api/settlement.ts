import { env } from '../config/env';
import { fetchJson } from './http';
import type { Settlement, Transaction } from './types';

export function listHeldTransactions(): Promise<Transaction[]> {
  return fetchJson<Transaction[]>(`${env.settlementApiUrl}/transactions?status=held`);
}

export function approveTransaction(id: string): Promise<void> {
  return fetchJson<void>(`${env.settlementApiUrl}/transactions/${id}/approve`, { method: 'POST' });
}

export function rejectTransaction(id: string): Promise<void> {
  return fetchJson<void>(`${env.settlementApiUrl}/transactions/${id}/reject`, { method: 'POST' });
}

export function listSettlements(): Promise<Settlement[]> {
  return fetchJson<Settlement[]>(`${env.settlementApiUrl}/settlements`);
}
