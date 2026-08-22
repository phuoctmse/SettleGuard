import type { ReactNode } from 'react';
import { LinearGradient } from 'expo-linear-gradient';
import { StyleSheet, View, type ViewStyle } from 'react-native';

import { colors, spacing } from '../theme';

interface Props {
  children: ReactNode;
  contentStyle?: ViewStyle;
}

export function ScreenContainer({ children, contentStyle }: Props) {
  return (
    <LinearGradient colors={colors.backgroundGradient} style={styles.gradient}>
      <View style={[styles.content, contentStyle]}>{children}</View>
    </LinearGradient>
  );
}

const styles = StyleSheet.create({
  gradient: { flex: 1 },
  content: { flex: 1, paddingHorizontal: spacing.lg },
});
