import { useEffect, useRef } from "react";
import { Animated, Text, View } from "react-native";
import Svg, { Circle, Line, Polyline, Rect } from "react-native-svg";
import { C } from "../theme";
import type { GPSPoint } from "../lib/recorder";

// Desenha a forma do traçado (sem tiles de mapa — o mapa real é MapLibre).
export function TrackPreview({ points, size = 260 }: { points: GPSPoint[]; size?: number }) {
  const pad = 16;

  if (points.length < 2) return <Waiting size={size} />;

  const lats = points.map((p) => p.lat);
  const lons = points.map((p) => p.lon);
  const minLat = Math.min(...lats);
  const maxLat = Math.max(...lats);
  const minLon = Math.min(...lons);
  const maxLon = Math.max(...lons);
  const spanLat = maxLat - minLat || 1e-6;
  const spanLon = maxLon - minLon || 1e-6;
  const scale = Math.min((size - pad * 2) / spanLon, (size - pad * 2) / spanLat);
  const w = spanLon * scale;
  const h = spanLat * scale;
  const ox = (size - w) / 2;
  const oy = (size - h) / 2;

  const xy = (p: GPSPoint) => [ox + (p.lon - minLon) * scale, oy + (maxLat - p.lat) * scale];
  const pts = points.map(xy).map(([x, y]) => `${x.toFixed(1)},${y.toFixed(1)}`).join(" ");
  const [sx, sy] = xy(points[0]);
  const [ex, ey] = xy(points[points.length - 1]);

  return (
    <Svg width={size} height={size}>
      <Rect x={0} y={0} width={size} height={size} rx={16} fill={C.sunken} />
      <Grid size={size} />
      <Polyline points={pts} fill="none" stroke={C.runner} strokeWidth={3.5} strokeLinecap="round" strokeLinejoin="round" />
      <Circle cx={sx} cy={sy} r={5} fill={C.teal} />
      <Circle cx={ex} cy={ey} r={5} fill={C.coral} />
    </Svg>
  );
}

function Grid({ size }: { size: number }) {
  const step = size / 5;
  const lines = [];
  for (let i = 1; i < 5; i++) {
    lines.push(<Line key={`h${i}`} x1={0} y1={i * step} x2={size} y2={i * step} stroke={C.line} strokeWidth={0.5} />);
    lines.push(<Line key={`v${i}`} x1={i * step} y1={0} x2={i * step} y2={size} stroke={C.line} strokeWidth={0.5} />);
  }
  return <>{lines}</>;
}

function Waiting({ size }: { size: number }) {
  const pulse = useRef(new Animated.Value(0)).current;
  useEffect(() => {
    const loop = Animated.loop(
      Animated.sequence([
        Animated.timing(pulse, { toValue: 1, duration: 900, useNativeDriver: true }),
        Animated.timing(pulse, { toValue: 0, duration: 900, useNativeDriver: true }),
      ]),
    );
    loop.start();
    return () => loop.stop();
  }, [pulse]);

  return (
    <View
      style={{
        width: size,
        height: size,
        borderRadius: 16,
        backgroundColor: C.sunken,
        alignItems: "center",
        justifyContent: "center",
        overflow: "hidden",
      }}
    >
      <Svg width={size} height={size} style={{ position: "absolute" }}>
        <Grid size={size} />
      </Svg>
      <Animated.View
        style={{
          width: 18,
          height: 18,
          borderRadius: 9,
          backgroundColor: C.teal,
          opacity: pulse.interpolate({ inputRange: [0, 1], outputRange: [0.5, 1] }),
          transform: [{ scale: pulse.interpolate({ inputRange: [0, 1], outputRange: [1, 1.8] }) }],
        }}
      />
      <Text style={{ marginTop: 22, fontSize: 12, color: C.ink3, fontWeight: "600" }}>
        aguardando sinal de GPS…
      </Text>
    </View>
  );
}
