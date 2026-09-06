import { useEffect, useRef } from "react";
import { Animated, type StyleProp, type ViewStyle } from "react-native";
import { DUR, EASE } from "../theme";

/**
 * Entrada suave: aparece de opacity 0 + deslocado alguns px.
 * `delay` escalona itens de uma lista (index * 40 costuma ficar bom).
 */
export function FadeIn({
  children,
  style,
  delay = 0,
  from = 10,
}: {
  children: React.ReactNode;
  style?: StyleProp<ViewStyle>;
  delay?: number;
  from?: number;
}) {
  const t = useRef(new Animated.Value(0)).current;

  useEffect(() => {
    const a = Animated.timing(t, {
      toValue: 1,
      duration: DUR.lg,
      delay,
      easing: EASE.out,
      useNativeDriver: true,
    });
    a.start();
    return () => a.stop();
  }, [t, delay]);

  return (
    <Animated.View
      style={[
        style,
        {
          opacity: t,
          transform: [{ translateY: t.interpolate({ inputRange: [0, 1], outputRange: [from, 0] }) }],
        },
      ]}
    >
      {children}
    </Animated.View>
  );
}

/** Pulso contínuo — para pontos "ao vivo", selos, chamadas de ação. */
export function Pulse({
  children,
  style,
  min = 0.9,
  max = 1.06,
  duration = DUR.xl,
}: {
  children: React.ReactNode;
  style?: StyleProp<ViewStyle>;
  min?: number;
  max?: number;
  duration?: number;
}) {
  const v = useRef(new Animated.Value(0)).current;
  useEffect(() => {
    const loop = Animated.loop(
      Animated.sequence([
        Animated.timing(v, { toValue: 1, duration, easing: EASE.standard, useNativeDriver: true }),
        Animated.timing(v, { toValue: 0, duration, easing: EASE.standard, useNativeDriver: true }),
      ]),
    );
    loop.start();
    return () => loop.stop();
  }, [v, duration]);
  return (
    <Animated.View
      style={[style, { transform: [{ scale: v.interpolate({ inputRange: [0, 1], outputRange: [min, max] }) }] }]}
    >
      {children}
    </Animated.View>
  );
}
