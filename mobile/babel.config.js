module.exports = function (api) {
  api.cache(true);
  return {
    presets: ["babel-preset-expo"],
    // react-native-reanimated/plugin PRECISA ser o último.
    plugins: ["react-native-reanimated/plugin"],
  };
};
