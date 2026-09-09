export type AccountStatus = 'active' | 'suspended' | 'closed';

export interface Account {
  id: string;
  client_id: string;
  external_ref: string | null;
  status: AccountStatus;
  balance: number;
  created_at: string;
}

export interface LedgerEntry {
  id: string;
  transaction_id: string;
  account_id: string;
  direction: 'debit' | 'credit';
  amount: number;
  reason: string;
  created_at: string;
}

export interface Transaction {
  id: string;
  amount: number;
  score: number;
  decision: 'pass' | 'hold';
  status: 'pending_settlement' | 'held' | 'settled' | 'rejected';
  triggered_rules: string[];
  scored_at: string;
}

export interface Settlement {
  id: string;
  transaction_ids: string[];
  transaction_count: number;
  total_amount: number;
  created_at: string;
}

export interface Notification {
  id: string;
  type: 'risk_hold' | 'settlement_finalized';
  subject_id: string;
  payload: Record<string, unknown>;
  created_at: string;
}
