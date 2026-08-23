import { Text } from 'react-native';

import { colors, typography } from '../theme';

interface Props {
  amount: number;
}

export function BalanceDisplay({ amount }: Props) {
  const color = amount < 0 ? colors.danger : colors.success;
  return <Text style={[typography.display, { color }]}>Balance: {amount}</Text>;
}
