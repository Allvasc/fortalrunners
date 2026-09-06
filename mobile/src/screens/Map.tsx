import { StyleSheet, Text, TouchableOpacity, View } from "react-native";
import {
  Camera,
  FillLayer,
  LineLayer,
  MapView,
  ShapeSource,
} from "@maplibre/maplibre-react-native";
import { useQuery } from "@tanstack/react-query";
import type { NativeStackScreenProps } from "@react-navigation/native-stack";
import { api } from "../lib/api";
import { C } from "../theme";
import type { RootStack } from "../../App";

type Props = NativeStackScreenProps<RootStack, "Map">;

// Estilo demo do MapLibre (OSM, sem chave). Trocar pelo style JSON próprio
// (MapTiler/Protomaps) quando a chave de produção estiver configurada.
const STYLE = "https://demotiles.maplibre.org/style.json";
const FORTALEZA: [number, number] = [-38.523, -3.731];

export function Map({ navigation }: Props) {
  const terr = useQuery({
    queryKey: ["territories"],
    queryFn: api.territories,
  });
  const fc = terr.data ?? { type: "FeatureCollection" as const, features: [] };

  return (
    <View style={s.wrap}>
      <MapView style={s.map} mapStyle={STYLE} logoEnabled={false} attributionEnabled compassEnabled={false}>
        <Camera defaultSettings={{ centerCoordinate: FORTALEZA, zoomLevel: 12 }} />
        <ShapeSource id="territories" shape={fc as GeoJSON.FeatureCollection}>
          <FillLayer id="terr-fill" style={{ fillColor: C.runner, fillOpacity: 0.22 }} />
          <LineLayer id="terr-line" style={{ lineColor: C.runner, lineWidth: 2.5 }} />
        </ShapeSource>
      </MapView>

      <View style={s.overlay}>
        <Text style={s.stat}>{fc.features.length} quarteirões seus</Text>
        {fc.features.length === 0 && (
          <Text style={s.hint}>Corra em circuito para pintar o primeiro.</Text>
        )}
      </View>

      <TouchableOpacity style={s.fab} onPress={() => navigation.navigate("Recording")}>
        <Text style={s.fabText}>Iniciar corrida</Text>
      </TouchableOpacity>
    </View>
  );
}

const s = StyleSheet.create({
  wrap: { flex: 1, backgroundColor: C.bg },
  map: { flex: 1 },
  overlay: {
    position: "absolute",
    top: 16,
    left: 16,
    backgroundColor: C.surface,
    borderRadius: 12,
    borderWidth: 1,
    borderColor: C.line,
    padding: 12,
    gap: 4,
  },
  stat: { fontWeight: "800", color: C.ink },
  hint: { color: C.ink3, fontSize: 12, maxWidth: 200 },
  fab: {
    position: "absolute",
    bottom: 28,
    alignSelf: "center",
    backgroundColor: C.coral,
    paddingHorizontal: 28,
    paddingVertical: 16,
    borderRadius: 999,
  },
  fabText: { color: "#fff", fontWeight: "800", fontSize: 16 },
});
