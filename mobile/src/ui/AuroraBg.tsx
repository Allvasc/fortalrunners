import { useEffect, useRef } from "react";
import { Animated, StyleSheet, useWindowDimensions, View } from "react-native";
import Svg, { Circle, Defs, RadialGradient, Stop } from "react-native-svg";
import { C } from "../theme";

const ASvg = Animated.createAnimatedComponent(Svg);

/**
 * Fundo ambiente: manchas radiais suaves (teal / coral / gold) que derivam
 * devagar. Fica atrás do conteúdo, opacidade baixa — dá profundidade sem
 * atrapalhar a leitura. Sem arquivos de imagem.
 */
export function AuroraBg({ tint = "teal" }: { tint?: "teal" | "coral" | "gold" }) {
  const { width, height } = useWindowDimensions();
  const drift = useRef(new Animated.Value(0)).current;

  useEffect(() => {
    const loop = Animated.loop(
      Animated.sequence([
        Animated.timing(drift, { toValue: 1, duration: 14000, useNativeDriver: true }),
        Animated.timing(drift, { toValue: 0, duration: 14000, useNativeDriver: true }),
      ]),
    );
    loop.start();
    return () => loop.stop();
  }, [drift]);

  const a = { teal: C.teal, coral: C.coral, gold: C.gold }[tint];
  const b = tint === "teal" ? C.coral : C.teal;

  const t1 = drift.interpolate({ inputRange: [0, 1], outputRange: [0, 26] });
  const t2 = drift.interpolate({ inputRange: [0, 1], outputRange: [0, -34] });

  return (
    <View style={StyleSheet.absoluteFill} pointerEvents="none">
      <Animated.View style={[StyleSheet.absoluteFill, { transform: [{ translateY: t1 }] }]}>
        <Svg width={width} height={height}>
          <Defs>
            <RadialGradient id="g1" cx="50%" cy="50%" r="50%">
              <Stop offset="0" stopColor={a} stopOpacity={0.16} />
              <Stop offset="1" stopColor={a} stopOpacity={0} />
            </RadialGradient>
          </Defs>
          <Circle cx={width * 0.82} cy={height * 0.12} r={width * 0.7} fill="url(#g1)" />
        </Svg>
      </Animated.View>

      <Animated.View style={[StyleSheet.absoluteFill, { transform: [{ translateX: t2 }] }]}>
        <ASvg width={width} height={height}>
          <Defs>
            <RadialGradient id="g2" cx="50%" cy="50%" r="50%">
              <Stop offset="0" stopColor={b} stopOpacity={0.1} />
              <Stop offset="1" stopColor={b} stopOpacity={0} />
            </RadialGradient>
          </Defs>
          <Circle cx={width * 0.1} cy={height * 0.7} r={width * 0.8} fill="url(#g2)" />
        </ASvg>
      </Animated.View>
    </View>
  );
}
