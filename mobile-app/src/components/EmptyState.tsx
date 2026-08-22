import { StyleSheet, Text } from 'react-native';

import { colors, spacing, typography } from '../theme';

interface Props {
  status: 'loading' | 'error' | 'empty';
  message?: string;
}

export function EmptyState({ status, message }: Props) {
  const text = status === 'loading' ? 'Loading…' : (message ?? '');
  return (
    <Text style={[typography.body, styles.text, status === 'error' && styles.error]}>{text}</Text>
  );
}

const styles = StyleSheet.create({
  text: { padding: spacing.lg, color: colors.textSecondary },
  error: { color: colors.danger },
});
