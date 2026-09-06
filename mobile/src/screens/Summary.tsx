import { useEffect } from "react";
import { ScrollView, StyleSheet, Text, TouchableOpacity, View } from "react-native";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import type { NativeStackScreenProps } from "@react-navigation/native-stack";
import { api, connectEvents } from "../lib/api";
import { C } from "../theme";
import { area, clock, km, pace } from "../format";
import type { RootStack } from "../../App";

type Props = NativeStackScreenProps<RootStack, "Summary">;

export function Summary({ route, navigation }: Props) {
  const { runId } = route.params;
  const qc = useQueryClient();
  const q = useQuery({
    queryKey: ["run", runId],
    queryFn: () => api.run(runId),
    // o worker leva alguns segundos para processar o território
    refetchInterval: (d) => (d.state.data?.status === "processing" ? 3000 : false),
  });

  useEffect(() => {
    qc.invalidateQueries({ queryKey: ["runs"] });
    qc.invalidateQueries({ queryKey: ["lifetime"] });
  }, [q.data?.status, qc]);

  // WebSocket: quando o servidor avisa que a corrida processou, atualiza na hora.
  useEffect(() => {
    let close = () => {};
    connectEvents((e) => {
      if (e.type === "run.processed" && e.data?.run_id === runId) {
        qc.invalidateQueries({ queryKey: ["run", runId] });
        qc.invalidateQueries({ queryKey: ["lifetime"] });
      }
    }).then((c) => {
      close = c;
    });
    return () => close();
  }, [runId, qc]);

  const r = q.data;

  return (
    <ScrollView style={{ backgroundColor: C.bg }} contentContainerStyle={s.wrap}>
      <Text style={s.title}>Corrida salva</Text>

      <View style={s.hero}>
        <Text style={s.heroV}>{km(r?.distance_m)}</Text>
        <Text style={s.heroL}>km</Text>
      </View>

      <View style={s.grid}>
        <Cell v={clock(r?.moving_s)} l="tempo em movimento" />
        <Cell v={`${pace(r?.avg_pace_s)}/km`} l="pace médio" />
        <Cell v={`${r?.elevation_gain_m ?? 0} m`} l="elevação" />
        <Cell v={area(r?.territory_area_m2)} l="território ganho" />
        <Cell v={String(r?.new_blocks ?? 0)} l="quarteirões novos" />
        <Cell
          v={
            r?.status === "processing"
              ? "processando…"
              : r?.status === "flagged"
                ? "em revisão"
                : "válida"
          }
          l="status"
        />
      </View>

      {r?.status === "flagged" && (
        <Text style={s.note}>
          Essa corrida foi marcada para revisão da moderação. O território conta assim que for liberada.
        </Text>
      )}

      <TouchableOpacity style={s.btn} onPress={() => navigation.navigate("Home")}>
        <Text style={s.btnText}>Voltar ao início</Text>
      </TouchableOpacity>
    </ScrollView>
  );
}

function Cell({ v, l }: { v: string; l: string }) {
  return (
    <View style={s.cell}>
      <Text style={s.cellV}>{v}</Text>
      <Text style={s.cellL}>{l}</Text>
    </View>
  );
}

const s = StyleSheet.create({
  wrap: { padding: 20, gap: 16, paddingTop: 32 },
  title: { fontSize: 22, fontWeight: "800", color: C.ink, textAlign: "center" },
  hero: { alignItems: "center", marginVertical: 8 },
  heroV: { fontSize: 56, fontWeight: "800", color: C.ink, fontVariant: ["tabular-nums"] },
  heroL: { color: C.ink3 },
  grid: { flexDirection: "row", flexWrap: "wrap", gap: 10 },
  cell: { flexGrow: 1, minWidth: "45%", backgroundColor: C.surface, borderWidth: 1, borderColor: C.line, borderRadius: 12, padding: 14 },
  cellV: { fontSize: 18, fontWeight: "800", color: C.ink },
  cellL: { fontSize: 11, textTransform: "uppercase", color: C.ink3, marginTop: 2 },
  note: { color: C.ink2, fontSize: 13, backgroundColor: C.sunken, padding: 12, borderRadius: 10 },
  btn: { backgroundColor: C.teal, borderRadius: 12, paddingVertical: 15, alignItems: "center", marginTop: 8 },
  btnText: { color: "#fff", fontWeight: "800" },
});
