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
