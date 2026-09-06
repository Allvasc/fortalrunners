import { Image, StyleSheet, useWindowDimensions, View } from "react-native";
import { StatusBar } from "expo-status-bar";
import { C } from "../theme";

// Splash em tela cheia — o Android 12+ força a splash NATIVA a ser só um ícone
// centralizado, então cobrimos com a imagem inteira (100% da tela) assim que o
// JS monta e a seguramos por um tempo mínimo.
export function Splash() {
  const { width, height } = useWindowDimensions();
  return (
    <View style={[s.wrap, { width, height }]}>
      <StatusBar hidden />
      <Image source={require("../../assets/splash.png")} style={{ width, height }} resizeMode="cover" />
    </View>
  );
}

const s = StyleSheet.create({
  wrap: { position: "absolute", top: 0, left: 0, backgroundColor: C.bg },
});
