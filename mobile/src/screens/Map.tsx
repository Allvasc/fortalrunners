import { useEffect, useMemo, useRef, useState } from "react";
import { ScrollView, StyleSheet, Text, TouchableOpacity, View } from "react-native";
import * as Location from "expo-location";
import {
  Camera,
  CircleLayer,
  FillLayer,
  HeatmapLayer,
  LineLayer,
  MapView as MLRNMapView,
  ShapeSource,
  UserLocation,
} from "@maplibre/maplibre-react-native";
import { useQuery } from "@tanstack/react-query";
import type { NativeStackScreenProps } from "@react-navigation/native-stack";
import { api, type GeoFC } from "../lib/api";
import { C } from "../theme";
import { area } from "../format";
import type { RootStack } from "../../App";

type Props = NativeStackScreenProps<RootStack, "Map">;

const FORTALEZA: [number, number] = [-38.523, -3.731];
const RUNNER = "#08a6a0";
const EMPTY: GeoFC = { type: "FeatureCollection", features: [] };

// Estilos raster completos (sem chave) — MapLibre native carrega o style inteiro.
const rasterStyle = (id: string, tiles: string[], attribution: string) =>
  JSON.stringify({
    version: 8,
    sources: { [id]: { type: "raster", tiles, tileSize: 256, maxzoom: 19, attribution } },
    layers: [
      { id: "bg", type: "background", paint: { "background-color": "#e9e4d8" } },
      { id, type: "raster", source: id },
    ],
  });

const STYLE_OSM = rasterStyle("osm", ["https://tile.openstreetmap.org/{z}/{x}/{y}.png"], "© OpenStreetMap");
const STYLE_SAT = rasterStyle(
  "sat",
  ["https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}"],
  "Esri, Maxar",
);

type LKey = "heat" | "landmarks" | "routes" | "pois" | "risk" | "hazards";

export function Map({ navigation }: Props) {
  const cam = useRef<React.ComponentRef<typeof Camera>>(null);
  const [collapsed, setCollapsed] = useState(false);
  const [base, setBase] = useState<"osm" | "sat">("osm");
  const [scope, setScope] = useState<"me" | "friends">("me");
  const [heatScope, setHeatScope] = useState<"me" | "friends" | "city">("me");
  const [hasLoc, setHasLoc] = useState(false);
  const [on, setOn] = useState<Record<LKey, boolean>>({
    heat: false, landmarks: false, routes: false, pois: false, risk: true, hazards: false,
  });
  const toggle = (k: LKey) => setOn((s) => ({ ...s, [k]: !s[k] }));

  useEffect(() => {
    Location.requestForegroundPermissionsAsync()
      .then((r) => setHasLoc(r.status === "granted"))
      .catch(() => {});
  }, []);

  const terr = useQuery({ queryKey: ["territories", scope], queryFn: () => api.territoriesScope(scope) });
  const heat = useQuery({ queryKey: ["heatmap", heatScope], queryFn: () => api.heatmap(heatScope), enabled: on.heat });
  const lmk = useQuery({ queryKey: ["landmarks"], queryFn: api.landmarksProgress, enabled: on.landmarks });
  const poi = useQuery({ queryKey: ["amenities"], queryFn: api.amenities, enabled: on.pois });
  const routes = useQuery({ queryKey: ["routesGeo"], queryFn: api.routes, enabled: on.routes });
  const risk = useQuery({ queryKey: ["riskZones"], queryFn: api.riskZones, enabled: on.risk });
  const haz = useQuery({ queryKey: ["hazards"], queryFn: () => api.hazards(), enabled: on.hazards });
  const weather = useQuery({ queryKey: ["weather"], queryFn: api.weather });

  const safe = (g?: GeoFC): GeoFC => (g && Array.isArray(g.features) ? g : EMPTY);
  const fc = safe(terr.data);

  const landmarksFC = useMemo<GeoFC>(() => ({
    type: "FeatureCollection",
    features: (lmk.data?.landmarks ?? []).map((l) => ({
      type: "Feature",
      geometry: { type: "Point", coordinates: [l.lng, l.lat] },
      properties: { name: l.name, done: l.checked_in ? 1 : 0 },
    })),
  }), [lmk.data]);

  const poisFC = useMemo<GeoFC>(() => ({
    type: "FeatureCollection",
    features: (poi.data?.amenities ?? []).map((p) => ({
      type: "Feature",
      geometry: { type: "Point", coordinates: [p.lng, p.lat] },
      properties: { name: p.name },
    })),
  }), [poi.data]);

  const routesFC = useMemo<GeoFC>(() => ({
    type: "FeatureCollection",
    features: (routes.data?.routes ?? [])
      .filter((r) => !!r.geojson)
      .map((r) => {
        try {
          return { type: "Feature" as const, geometry: JSON.parse(r.geojson as string), properties: { name: r.name } };
        } catch {
          return null;
        }
      })
      .filter(Boolean) as GeoJSON.Feature[],
  }), [routes.data]);

  const hazFC = useMemo<GeoFC>(() => ({
    type: "FeatureCollection",
    features: (haz.data?.hazards ?? []).map((h) => ({
      type: "Feature",
      geometry: { type: "Point", coordinates: [h.lng, h.lat] },
      properties: { type: h.type },
    })),
  }), [haz.data]);

  const count = fc.features.length;
  const totalArea = fc.features.reduce((sum, f) => sum + Number((f.properties as any)?.area_m2 ?? 0), 0);

  const centerOnMe = async () => {
    try {
      let ok = hasLoc;
      if (!ok) {
        const r = await Location.requestForegroundPermissionsAsync();
        ok = r.status === "granted";
        setHasLoc(ok);
      }
      if (!ok) return;
      const pos = await Location.getCurrentPositionAsync({ accuracy: Location.Accuracy.Balanced });
      cam.current?.setCamera({
        centerCoordinate: [pos.coords.longitude, pos.coords.latitude],
        zoomLevel: 15,
        animationDuration: 700,
      });
    } catch {}
  };

  return (
    <View style={s.wrap}>
      <MLRNMapView
        style={s.map}
        mapStyle={base === "osm" ? STYLE_OSM : STYLE_SAT}
        logoEnabled={false}
        attributionEnabled
        compassEnabled={false}
      >
        <Camera ref={cam} defaultSettings={{ centerCoordinate: FORTALEZA, zoomLevel: 11.5 }} />
        {hasLoc && <UserLocation visible androidRenderMode="compass" />}

        {on.risk && (
          <ShapeSource id="risk" shape={safe(risk.data)}>
            <FillLayer id="risk-fill" style={{ fillColor: "#c7402b", fillOpacity: 0.16 }} />
            <LineLayer id="risk-line" style={{ lineColor: "#c7402b", lineWidth: 1.4, lineDasharray: [2, 2] }} />
          </ShapeSource>
        )}
        {on.heat && (
          <ShapeSource id="heat" shape={safe(heat.data)}>
            <HeatmapLayer id="heat-l" style={{ heatmapRadius: 22, heatmapOpacity: 0.75 }} />
          </ShapeSource>
        )}
        {on.routes && (
          <ShapeSource id="routes" shape={routesFC}>
            <LineLayer id="routes-l" style={{ lineColor: "#6E5AA6", lineWidth: 3, lineCap: "round" }} />
          </ShapeSource>
        )}

        <ShapeSource id="terr" shape={fc}>
          <FillLayer id="terr-fill" style={{ fillColor: ["coalesce", ["get", "color_hex"], RUNNER], fillOpacity: 0.22 }} />
          <LineLayer id="terr-line" style={{ lineColor: ["coalesce", ["get", "color_hex"], RUNNER], lineWidth: 2.5 }} />
        </ShapeSource>

        {on.landmarks && (
          <ShapeSource id="lmk" shape={landmarksFC}>
            <CircleLayer
              id="lmk-l"
              style={{
                circleRadius: 6,
                circleColor: ["case", ["==", ["get", "done"], 1], C.good, C.coral],
                circleStrokeColor: "#fff",
                circleStrokeWidth: 2,
              }}
            />
          </ShapeSource>
        )}
        {on.pois && (
          <ShapeSource id="pois" shape={poisFC}>
            <CircleLayer id="pois-l" style={{ circleRadius: 5, circleColor: C.teal, circleStrokeColor: "#fff", circleStrokeWidth: 2 }} />
          </ShapeSource>
        )}
        {on.hazards && (
          <ShapeSource id="haz" shape={hazFC}>
            <CircleLayer id="haz-l" style={{ circleRadius: 6, circleColor: "#C98F2E", circleStrokeColor: "#fff", circleStrokeWidth: 2 }} />
          </ShapeSource>
        )}
      </MLRNMapView>

      <TouchableOpacity style={s.gps} onPress={centerOnMe}>
        <Text style={[s.gpsIcon, hasLoc && { color: C.teal }]}>◎</Text>
      </TouchableOpacity>

      {collapsed ? (
        <TouchableOpacity style={s.fab} onPress={() => setCollapsed(false)}>
          <Text style={s.fabIcon}>▨</Text>
        </TouchableOpacity>
      ) : (
        <View style={s.panel}>
          <View style={s.panelHead}>
            <Text style={s.panelStat}>
              <Text style={{ fontWeight: "800" }}>{count}</Text> quart. ·{" "}
              <Text style={{ fontWeight: "800" }}>{area(totalArea)}</Text>
            </Text>
            <TouchableOpacity onPress={() => setCollapsed(true)} hitSlop={10}>
              <Text style={s.min}>–</Text>
            </TouchableOpacity>
          </View>

          <ScrollView style={{ maxHeight: 300 }} showsVerticalScrollIndicator={false}>
            <Text style={s.sec}>Base</Text>
            <View style={s.row}>
              <Seg label="Mapa" active={base === "osm"} onPress={() => setBase("osm")} />
              <Seg label="Satélite" active={base === "sat"} onPress={() => setBase("sat")} />
            </View>

            <Text style={s.sec}>Território</Text>
            <View style={s.row}>
              <Seg label="Meu" active={scope === "me"} onPress={() => setScope("me")} />
              <Seg label="Amigos" active={scope === "friends"} onPress={() => setScope("friends")} />
            </View>

            <Text style={s.sec}>Camadas</Text>
            <Layer label="Mapa de calor" active={on.heat} onPress={() => toggle("heat")} />
            {on.heat && (
              <View style={[s.row, { marginTop: 4 }]}>
                {(["me", "friends", "city"] as const).map((v) => (
                  <Seg key={v} label={v === "me" ? "eu" : v === "friends" ? "rede" : "cidade"} active={heatScope === v} onPress={() => setHeatScope(v)} />
                ))}
              </View>
            )}
            <Layer label="Marcos" active={on.landmarks} onPress={() => toggle("landmarks")} />
            <Layer label="Rotas" active={on.routes} onPress={() => toggle("routes")} />
            <Layer label="Bebedouros / banheiros" active={on.pois} onPress={() => toggle("pois")} />
            <Layer label="Zonas de risco" active={on.risk} onPress={() => toggle("risk")} />
            <Layer label="Alertas na via" active={on.hazards} onPress={() => toggle("hazards")} />

            {weather.data && (
              <View style={s.weather}>
                <Text style={s.wTitle}>
                  {weather.data.city} · {weather.data.temp_c}°C
                </Text>
                <Text style={s.wSub}>
                  Sensação {weather.data.feels_like_c}° · UV {weather.data.uv_index} · Vento {weather.data.wind_kmh} km/h
                </Text>
              </View>
            )}
            {!hasLoc && (
              <Text style={s.hint}>Ative a permissão de localização para ver sua posição no mapa.</Text>
            )}
            {count === 0 && !terr.isLoading && (
              <Text style={s.hint}>Mapa em branco — corra em circuito para pintar o primeiro quarteirão.</Text>
            )}
          </ScrollView>
        </View>
      )}

      <TouchableOpacity style={s.run} onPress={() => navigation.navigate("Recording")}>
        <Text style={s.runText}>Iniciar corrida</Text>
      </TouchableOpacity>
    </View>
  );
}

function Seg({ label, active, onPress }: { label: string; active: boolean; onPress: () => void }) {
  return (
    <TouchableOpacity style={[s.seg, active && s.segOn]} onPress={onPress}>
      <Text style={[s.segTxt, active && s.segTxtOn]}>{label}</Text>
    </TouchableOpacity>
  );
}

function Layer({ label, active, onPress }: { label: string; active: boolean; onPress: () => void }) {
  return (
    <TouchableOpacity style={s.layer} onPress={onPress}>
      <View style={[s.dot, active && s.dotOn]} />
      <Text style={[s.layerTxt, active && { color: C.ink, fontWeight: "700" }]}>{label}</Text>
    </TouchableOpacity>
  );
}

const s = StyleSheet.create({
  wrap: { flex: 1, backgroundColor: C.bg },
  map: { flex: 1 },
  gps: {
    position: "absolute", top: 16, right: 16, width: 44, height: 44, borderRadius: 22,
    backgroundColor: C.surface, borderWidth: 1, borderColor: C.line, alignItems: "center", justifyContent: "center",
  },
  gpsIcon: { fontSize: 22, color: C.ink3 },
  fab: {
    position: "absolute", left: 16, bottom: 96, width: 52, height: 52, borderRadius: 26,
    backgroundColor: C.surface, borderWidth: 1, borderColor: C.line, alignItems: "center", justifyContent: "center",
  },
  fabIcon: { fontSize: 22, color: C.ink },
  panel: {
    position: "absolute", top: 16, left: 16, width: 250, backgroundColor: C.surface,
    borderRadius: 14, borderWidth: 1, borderColor: C.line, padding: 12,
  },
  panelHead: { flexDirection: "row", justifyContent: "space-between", alignItems: "center", marginBottom: 6 },
  panelStat: { fontSize: 13, color: C.ink2 },
  min: { fontSize: 20, color: C.ink3, paddingHorizontal: 6, lineHeight: 20 },
  sec: { fontSize: 10, fontWeight: "800", letterSpacing: 0.5, textTransform: "uppercase", color: C.ink3, marginTop: 10, marginBottom: 4 },
  row: { flexDirection: "row", gap: 5 },
  seg: { flex: 1, paddingVertical: 6, borderRadius: 7, backgroundColor: C.sunken, alignItems: "center" },
  segOn: { backgroundColor: C.coral },
  segTxt: { fontSize: 11, fontWeight: "700", color: C.ink2 },
  segTxtOn: { color: "#fff" },
  layer: { flexDirection: "row", alignItems: "center", gap: 8, paddingVertical: 6 },
  dot: { width: 12, height: 12, borderRadius: 6, borderWidth: 1.5, borderColor: C.line, backgroundColor: "transparent" },
  dotOn: { backgroundColor: C.teal, borderColor: C.teal },
  layerTxt: { fontSize: 13, color: C.ink3 },
  weather: { marginTop: 10, backgroundColor: C.sunken, borderRadius: 8, padding: 8 },
  wTitle: { fontSize: 12, fontWeight: "800", color: C.tealDeep },
  wSub: { fontSize: 11, color: C.ink3, marginTop: 2 },
  hint: { fontSize: 11, color: C.ink3, marginTop: 10, lineHeight: 15 },
  run: {
    position: "absolute", bottom: 24, alignSelf: "center", backgroundColor: C.coral,
    paddingHorizontal: 28, paddingVertical: 15, borderRadius: 999,
  },
  runText: { color: "#fff", fontWeight: "800", fontSize: 16 },
});
