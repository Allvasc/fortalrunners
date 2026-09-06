import { Image, StyleSheet, View } from "react-native";
import { C } from "../theme";

// Splash em tela cheia — o Android 12+ força a splash NATIVA a ser só um ícone
// centralizado, então cobrimos com a imagem inteira assim que o JS monta.
export function Splash() {
  return (
    <View style={s.wrap}>
      <Image source={require("../../assets/splash.png")} style={StyleSheet.absoluteFill} resizeMode="cover" />
    </View>
  );
}

const s = StyleSheet.create({
  wrap: { flex: 1, backgroundColor: C.bg },
});
