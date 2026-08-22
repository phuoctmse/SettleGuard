function requireEnv(name: string, value: string | undefined): string {
  if (!value) {
    throw new Error(`${name} environment variable is required`);
  }
  return value;
}

export const env = {
  accountsApiUrl: requireEnv('EXPO_PUBLIC_ACCOUNTS_API_URL', process.env.EXPO_PUBLIC_ACCOUNTS_API_URL),
  ledgerApiUrl: requireEnv('EXPO_PUBLIC_LEDGER_API_URL', process.env.EXPO_PUBLIC_LEDGER_API_URL),
  settlementApiUrl: requireEnv('EXPO_PUBLIC_SETTLEMENT_API_URL', process.env.EXPO_PUBLIC_SETTLEMENT_API_URL),
  notificationApiUrl: requireEnv('EXPO_PUBLIC_NOTIFICATION_API_URL', process.env.EXPO_PUBLIC_NOTIFICATION_API_URL),
};
