import { colors } from './colors';
import { typography } from './typography';
import { spacing } from './spacing';

it('exposes the accent color used throughout the app', () => {
  expect(colors.accent).toBe('#E8541F');
});

it('exposes a display typography style for balance figures', () => {
  expect(typography.display.fontSize).toBe(32);
  expect(typography.display.fontWeight).toBe('800');
});

it('exposes the spacing scale', () => {
  expect(spacing.lg).toBe(16);
  expect(spacing.cardRadius).toBe(14);
});
