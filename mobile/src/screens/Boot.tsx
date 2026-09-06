import { useEffect, useRef } from "react";
import { Animated, Easing, StyleSheet, Text, View } from "react-native";
import Svg, { Circle, Path } from "react-native-svg";
import { AuroraBg } from "../ui/AuroraBg";
import { C, EASE } from "../theme";

const APath = Animated.createAnimatedComponent(Path);
const ACircle = Animated.createAnimatedComponent(Circle);

// Traçado que "pinta" um quarteirão — a mecânica central do jogo.
const ROUTE = "M20 74 L20 26 L74 26 L74 74 Z";
const LEN = 192;

/**
 * Tela de abertura animada: mostra enquanto o app confere o login.
 * Marca pulsando, traçado do quarteirão sendo desenhado, wordmark surgindo.
 */
export function Boot() {
  const draw = useRef(new Animated.Value(0)).current;
  const mark = useRef(new Animated.Value(0)).current;
  const word = useRef(new Animated.Value(0)).current;
  const runner = useRef(new Animated.Value(0)).current;

  useEffect(() => {
    Animated.sequence([
      Animated.timing(mark, { toValue: 1, duration: 420, easing: EASE.spring, useNativeDriver: true }),
      Animated.timing(word, { toValue: 1, duration: 320, easing: EASE.out, useNativeDriver: true }),
    ]).start();

    Animated.loop(
      Animated.timing(draw, { toValue: 1, duration: 1600, easing: Easing.inOut(Easing.ease), useNativeDriver: true }),
    ).start();

    Animated.loop(
      Animated.timing(runner, { toValue: 1, duration: 2400, easing: Easing.linear, useNativeDriver: true }),
    ).start();
  }, [draw, mark, word, runner]);

  const dashOffset = draw.interpolate({ inputRange: [0, 1], outputRange: [LEN, 0] });
  const markScale = mark.interpolate({ inputRange: [0, 1], outputRange: [0.6, 1] });

  // corredor percorrendo o perímetro do quadrado
  const rx = runner.interpolate({ inputRange: [0, 0.25, 0.5, 0.75, 1], outputRange: [20, 74, 74, 20, 20] });
  const ry = runner.interpolate({ inputRange: [0, 0.25, 0.5, 0.75, 1], outputRange: [74, 74, 26, 26, 74] });

  return (
    <View style={s.wrap}>
      <AuroraBg tint="teal" />

      <Animated.View style={{ transform: [{ scale: markScale }], opacity: mark }}>
        <View style={s.mark}>
          <Svg width={94} height={94} viewBox="0 0 94 94">
            <APath
              d={ROUTE}
              stroke="#fff"
              strokeWidth={5}
              strokeLinecap="round"
              strokeLinejoin="round"
              fill="none"
              strokeDasharray={LEN}
              strokeDashoffset={dashOffset}
            />
            <ACircle cx={rx} cy={ry} r={5} fill={C.gold} />
          </Svg>
        </View>
      </Animated.View>

      <Animated.Text
        style={[
          s.word,
          { opacity: word, transform: [{ translateY: word.interpolate({ inputRange: [0, 1], outputRange: [8, 0] }) }] },
        ]}
      >
        FortalRunners
      </Animated.Text>
      <Animated.Text style={[s.tag, { opacity: word }]}>Corra. Conquiste Fortaleza.</Animated.Text>

      <View style={s.barTrack}>
        <Animated.View
          style={[
            s.barFill,
            {
              transform: [
                {
                  translateX: draw.interpolate({ inputRange: [0, 1], outputRange: [-120, 120] }),
                },
              ],
            },
          ]}
        />
      </View>
    </View>
  );
}

const s = StyleSheet.create({
  wrap: { flex: 1, backgroundColor: C.bg, alignItems: "center", justifyContent: "center", gap: 14 },
  mark: {
    width: 94,
    height: 94,
    borderRadius: 24,
    backgroundColor: C.teal,
    alignItems: "center",
    justifyContent: "center",
    shadowColor: C.tealDeep,
    shadowOpacity: 0.3,
    shadowRadius: 20,
    shadowOffset: { width: 0, height: 8 },
    elevation: 8,
  },
  word: { fontSize: 26, fontWeight: "800", color: C.ink, marginTop: 8, letterSpacing: 0.3 },
  tag: { fontSize: 13, color: C.ink3 },
  barTrack: {
    width: 120,
    height: 3,
    borderRadius: 2,
    backgroundColor: C.line,
    overflow: "hidden",
    marginTop: 20,
  },
  barFill: { width: 60, height: "100%", backgroundColor: C.teal, borderRadius: 2 },
});
