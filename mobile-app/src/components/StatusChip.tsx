import { StyleSheet, Text, View } from 'react-native';

import { colors, spacing, typography } from '../theme';

const STATUS_COLORS: Record<string, { bg: string; fg: string }> = {
  held: { bg: colors.warningMuted, fg: colors.warning },
  pending_settlement: { bg: colors.neutralMuted, fg: colors.neutral },
  settled: { bg: colors.successMuted, fg: colors.success },
  rejected: { bg: colors.dangerMuted, fg: colors.danger },
  risk_hold: { bg: colors.warningMuted, fg: colors.warning },
  settlement_finalized: { bg: colors.successMuted, fg: colors.success },
};

interface Props {
  status: string;
}

export function StatusChip({ status }: Props) {
  const tone = STATUS_COLORS[status] ?? { bg: colors.neutralMuted, fg: colors.neutral };
  return (
    <View style={[styles.chip, { backgroundColor: tone.bg }]}>
      <Text style={[typography.chip, { color: tone.fg }]}>{status}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  chip: {
    alignSelf: 'flex-start',
    borderRadius: 20,
    paddingHorizontal: spacing.sm,
    paddingVertical: 3,
  },
});
