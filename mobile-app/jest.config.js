module.exports = {
  preset: 'jest-expo',
  setupFiles: ['./jest.setup-env.js'],
  setupFilesAfterEnv: ['@testing-library/jest-native/extend-expect'],
  transformIgnorePatterns: [
    'node_modules/(?!((jest-)?react-native|@react-native(-community)?|expo(nent)?|@expo(nent)?/.*|@react-navigation/.*|expo-modules-core)/)',
  ],
  // @tanstack/react-query's QueryClient schedules a garbage-collection
  // timer (default gcTime 5 min) that outlives the test, so the worker
  // process never exits on its own once a screen using useQuery is
  // rendered. Tests pass either way; this just lets `npm test` return
  // instead of hanging until the timer fires.
  forceExit: true,
};
