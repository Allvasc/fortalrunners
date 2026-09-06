import { View } from "react-native";
import Svg, { Circle, Polyline, Rect } from "react-native-svg";
import { C } from "../theme";
import type { GPSPoint } from "../lib/recorder";

// Desenha a forma do traçado (sem tiles de mapa — o mapa real é MapLibre, Fase 2).
export function TrackPreview({ points, size = 260 }: { points: GPSPoint[]; size?: number }) {
  const pad = 16;
  if (points.length < 2) {
    return (
      <View style={{ width: size, height: size, borderRadius: 16, backgroundColor: C.sunken }} />
    );
  }
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

  const xy = (p: GPSPoint) => [
    ox + (p.lon - minLon) * scale,
    oy + (maxLat - p.lat) * scale, // y invertido
  ];
  const pts = points.map(xy).map(([x, y]) => `${x.toFixed(1)},${y.toFixed(1)}`).join(" ");
  const [sx, sy] = xy(points[0]);
  const [ex, ey] = xy(points[points.length - 1]);

  return (
    <Svg width={size} height={size}>
      <Rect x={0} y={0} width={size} height={size} rx={16} fill={C.sunken} />
      <Polyline
        points={pts}
        fill="none"
        stroke={C.runner}
        strokeWidth={3.5}
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <Circle cx={sx} cy={sy} r={5} fill={C.teal} />
      <Circle cx={ex} cy={ey} r={5} fill={C.coral} />
    </Svg>
  );
}
