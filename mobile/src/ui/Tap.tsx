import { useRef } from "react";
import {
  Animated,
  Pressable,
  type PressableProps,
  type StyleProp,
  type ViewStyle,
} from "react-native";
import { DUR, EASE } from "../theme";

type Props = PressableProps & {
  children: React.ReactNode;
  style?: StyleProp<ViewStyle>;
  /** quão fundo o toque "afunda" (0.96 = leve, 0.92 = forte) */
  to?: number;
  haptic?: boolean;
};

/**
 * Botão/área tocável com micro-interação: encolhe + esmaece no press-in,
 * volta com mola no press-out. Usar em todo CTA, card e item de lista.
 */
export function Tap({ children, style, to = 0.96, onPressIn, onPressOut, ...rest }: Props) {
  const scale = useRef(new Animated.Value(1)).current;
  const opacity = useRef(new Animated.Value(1)).current;

  const animate = (s: number, o: number, dur: number, easing: typeof EASE.standard) => {
    Animated.parallel([
      Animated.timing(scale, { toValue: s, duration: dur, easing, useNativeDriver: true }),
      Animated.timing(opacity, { toValue: o, duration: dur, easing, useNativeDriver: true }),
    ]).start();
  };

  return (
    <Pressable
      onPressIn={(e) => {
        animate(to, 0.85, DUR.xs, EASE.standard);
        onPressIn?.(e);
      }}
      onPressOut={(e) => {
        animate(1, 1, DUR.md, EASE.spring);
        onPressOut?.(e);
      }}
      {...rest}
    >
      <Animated.View style={[style, { transform: [{ scale }], opacity }]}>{children}</Animated.View>
    </Pressable>
  );
}
