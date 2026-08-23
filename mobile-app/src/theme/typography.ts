import type { TextStyle } from 'react-native';

type TypographyKey = 'display' | 'heading' | 'label' | 'body' | 'caption' | 'chip';

export const typography: Record<TypographyKey, TextStyle> = {
  display: { fontSize: 32, fontWeight: '800', letterSpacing: -1 },
  heading: { fontSize: 17, fontWeight: '700', letterSpacing: -0.2 },
  label: { fontSize: 12, fontWeight: '700', letterSpacing: 0.5, textTransform: 'uppercase' },
  body: { fontSize: 14, fontWeight: '500' },
  caption: { fontSize: 12, fontWeight: '500' },
  chip: { fontSize: 10, fontWeight: '700', letterSpacing: 0.3 },
};
