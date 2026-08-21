import { env } from '../config/env';
import { fetchJson } from './http';

export interface Transaction {
  id: string;
  amount: number;
  score: number;
  decision: 'pass' | 'hold';
  status: 'pending_settlement' | 'held' | 'settled' | 'rejected';
  triggered_rules: string[];
  scored_at: string;
}

export function listHeldTransactions(): Promise<Transaction[]> {
  return fetchJson<Transaction[]>(`${env.settlementApiUrl}/transactions?status=held`);
}

export function approveTransaction(id: string): Promise<void> {
  return fetchJson<void>(`${env.settlementApiUrl}/transactions/${id}/approve`, { method: 'POST' });
}

export function rejectTransaction(id: string): Promise<void> {
  return fetchJson<void>(`${env.settlementApiUrl}/transactions/${id}/reject`, { method: 'POST' });
}
