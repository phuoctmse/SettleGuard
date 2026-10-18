import { StyleSheet, Text } from 'react-native';

import { colors, spacing, typography } from '../theme';

interface Props {
  loading: boolean;
  failed: boolean;
}

export function LoadMoreFooter({ loading, failed }: Props) {
  if (loading) {
    return <Text style={[typography.caption, styles.text]}>Loading more…</Text>;
  }
  if (failed) {
    return (
      <Text style={[typography.caption, styles.text, styles.error]}>Failed to load more.</Text>
    );
  }
  return null;
}

const styles = StyleSheet.create({
  text: { padding: spacing.md, textAlign: 'center', color: colors.textSecondary },
  error: { color: colors.danger },
});
