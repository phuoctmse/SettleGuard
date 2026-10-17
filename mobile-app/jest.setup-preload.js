// react-native exports its components through lazy getters: `import {
// FlatList } from 'react-native'` does not load FlatList, the first
// `<FlatList>` rendered does. In a test that first render happens inside
// the first `it` of each file, so that test pays for requiring (and, on a
// cold Jest cache, Babel-transforming) the whole component tree. Measured
// on a cold cache with every worker transforming at once, touching these
// getters took ~6s, past Jest's 5s per-test timeout, which is why the
// first screen test in each file failed on a fresh checkout and passed on
// every rerun. Touching them here moves that one-time module load into
// per-file setup, where no test timeout applies.
//
// Keep this list in step with what src/ imports from 'react-native'.
// ScrollView is not imported by src/ directly: FlatList's VirtualizedList
// pulls it through the same lazy getter the first time a list renders
// rows, which on a cold cache was another ~1.7s inside the test.
const RN = require('react-native');

void [RN.FlatList, RN.Pressable, RN.ScrollView, RN.StyleSheet, RN.Text, RN.TextInput, RN.View];
