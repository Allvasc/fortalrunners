import { ScrollView, StyleSheet, Text, TouchableOpacity, View } from "react-native";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import type { NativeStackScreenProps } from "@react-navigation/native-stack";
import { api, type PublicUser } from "../lib/api";
import { C } from "../theme";
import { area, clock, km, pace } from "../format";
import type { RootStack } from "../../App";

type Props = NativeStackScreenProps<RootStack, "Home">;

export function Home({ navigation }: Props) {
  const qc = useQueryClient();
  const me = useQuery<PublicUser>({ queryKey: ["me"], queryFn: api.me });
  const life = useQuery({ queryKey: ["lifetime"], queryFn: api.lifetime });
  const runs = useQuery({ queryKey: ["runs"], queryFn: () => api.runs(8) });

  return (
    <ScrollView style={{ backgroundColor: C.bg }} contentContainerStyle={s.wrap}>
      <View style={s.head}>
        <View>
          <Text style={s.hello}>Olá, {me.data?.username ?? "corredor"}</Text>
          <Text style={s.muted}>{me.data?.athlete_id}</Text>
        </View>
        <TouchableOpacity
          onPress={async () => {
            await api.logout();
            qc.clear();
          }}
        >
          <Text style={{ color: C.ink3 }}>Sair</Text>
        </TouchableOpacity>
      </View>

      <View style={s.tiles}>
        <Tile v={String(life.data?.run_count ?? 0)} l="corridas" />
        <Tile v={km(life.data?.total_distance_m)} l="km totais" />
        <Tile v={area(life.data?.territory_area_m2)} l="território" />
        <Tile v={`${life.data?.current_streak_days ?? 0} d`} l="sequência" />
      </View>

      <TouchableOpacity style={s.cta} onPress={() => navigation.navigate("Recording")}>
        <Text style={s.ctaText}>Iniciar corrida</Text>
      </TouchableOpacity>
      <TouchableOpacity style={s.ctaGhost} onPress={() => navigation.navigate("Map")}>
        <Text style={s.ctaGhostText}>Ver meu mapa</Text>
      </TouchableOpacity>

      <Text style={s.section}>Últimas corridas</Text>
      {runs.data?.runs.length === 0 && <Text style={s.muted}>Nenhuma ainda. Bora?</Text>}
      {runs.data?.runs.map((r) => (
        <TouchableOpacity key={r.id} style={s.runRow} onPress={() => navigation.navigate("Summary", { runId: r.id })}>
          <View>
            <Text style={s.runDate}>
              {new Date(r.started_at).toLocaleDateString("pt-BR", { day: "2-digit", month: "short" })}
            </Text>
            <Text style={s.muted}>
              {r.status === "flagged" ? "em revisão" : r.status === "processing" ? "processando…" : `${area(r.territory_area_m2)} · ${r.new_blocks} quart.`}
            </Text>
          </View>
          <View style={{ alignItems: "flex-end" }}>
            <Text style={s.runKm}>{km(r.distance_m)} km</Text>
            <Text style={s.muted}>
              {pace(r.avg_pace_s)}/km · {clock(r.moving_s)}
            </Text>
          </View>
        </TouchableOpacity>
      ))}
    </ScrollView>
  );
}

function Tile({ v, l }: { v: string; l: string }) {
  return (
    <View style={s.tile}>
      <Text style={s.tileV}>{v}</Text>
      <Text style={s.tileL}>{l}</Text>
    </View>
  );
}

const s = StyleSheet.create({
  wrap: { padding: 20, gap: 16, paddingBottom: 40 },
  head: { flexDirection: "row", justifyContent: "space-between", alignItems: "flex-start", marginTop: 8 },
  hello: { fontSize: 22, fontWeight: "800", color: C.ink },
  muted: { color: C.ink3, fontSize: 13 },
  tiles: { flexDirection: "row", flexWrap: "wrap", gap: 10 },
  tile: { flexGrow: 1, minWidth: "45%", backgroundColor: C.surface, borderRadius: 12, borderWidth: 1, borderColor: C.line, padding: 12 },
  tileV: { fontSize: 18, fontWeight: "800", color: C.ink },
  tileL: { fontSize: 11, textTransform: "uppercase", color: C.ink3, marginTop: 2 },
  cta: { backgroundColor: C.coral, borderRadius: 14, paddingVertical: 18, alignItems: "center" },
  ctaText: { color: "#fff", fontWeight: "800", fontSize: 17 },
  ctaGhost: { borderWidth: 1.5, borderColor: C.line, borderRadius: 14, paddingVertical: 14, alignItems: "center" },
  ctaGhostText: { color: C.ink, fontWeight: "700" },
  section: { fontWeight: "700", color: C.ink, fontSize: 16, marginTop: 8 },
  runRow: { flexDirection: "row", justifyContent: "space-between", backgroundColor: C.surface, borderRadius: 12, borderWidth: 1, borderColor: C.line, padding: 14 },
  runDate: { fontWeight: "700", color: C.ink },
  runKm: { fontWeight: "700", color: C.ink, fontVariant: ["tabular-nums"] },
});
