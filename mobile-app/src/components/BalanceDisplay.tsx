import { Text } from 'react-native';

import { colors, typography } from '../theme';

interface Props {
  amount: number;
  testID?: string;
}

export function BalanceDisplay({ amount, testID }: Props) {
  const color = amount < 0 ? colors.danger : colors.success;
  return (
    <Text testID={testID} style={[typography.display, { color }]}>
      Balance: {amount}
    </Text>
  );
}
