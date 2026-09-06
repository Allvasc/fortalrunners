import { ActivityIndicator, FlatList, StyleSheet, Text, TouchableOpacity, View } from "react-native";
import { useQuery } from "@tanstack/react-query";
import type { NativeStackScreenProps } from "@react-navigation/native-stack";
import { api } from "../lib/api";
import { C } from "../theme";
import { area, clock, km, pace } from "../format";
import type { RootStack } from "../../App";

type Props = NativeStackScreenProps<RootStack, "History">;

const STATUS: Record<string, string> = {
  processing: "processando…",
  flagged: "em revisão",
  rejected: "rejeitada",
};

export function HistoryScreen({ navigation }: Props) {
  const q = useQuery({ queryKey: ["runs", "all"], queryFn: () => api.runs(60) });

  return (
    <View style={s.wrap}>
      <View style={s.head}>
        <Text style={s.title}>Histórico</Text>
        <Text style={s.sub}>Cada corrida com distância, pace, elevação e território conquistado.</Text>
      </View>

      {q.isLoading && <ActivityIndicator style={{ margin: 24 }} color={C.teal} />}

      <FlatList
        data={q.data?.runs ?? []}
        keyExtractor={(r) => r.id}
        contentContainerStyle={s.list}
        ListEmptyComponent={
          !q.isLoading ? <Text style={s.empty}>Nenhuma corrida ainda — registre uma pelo app.</Text> : null
        }
        renderItem={({ item: r }) => (
          <TouchableOpacity style={s.row} onPress={() => navigation.navigate("Summary", { runId: r.id })}>
            <View style={{ flex: 1 }}>
              <Text style={s.date}>
                {new Date(r.started_at).toLocaleDateString("pt-BR", {
                  day: "2-digit",
                  month: "short",
                  year: "numeric",
                })}
              </Text>
              <Text style={s.meta}>
                {STATUS[r.status] ?? `${area(r.territory_area_m2)} · ${r.new_blocks} quart.`}
              </Text>
            </View>
            <View style={{ alignItems: "flex-end" }}>
              <Text style={s.kmTxt}>{km(r.distance_m)} km</Text>
              <Text style={s.meta}>
                {pace(r.avg_pace_s)}/km · {clock(r.moving_s)}
              </Text>
            </View>
          </TouchableOpacity>
        )}
      />
    </View>
  );
}

const s = StyleSheet.create({
  wrap: { flex: 1, backgroundColor: C.bg },
  head: { padding: 16, borderBottomWidth: 1, borderBottomColor: C.line },
  title: { fontSize: 20, fontWeight: "800", color: C.ink },
  sub: { fontSize: 13, color: C.ink3, marginTop: 2, lineHeight: 18 },
  list: { padding: 16, gap: 8, paddingBottom: 40 },
  row: {
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
    backgroundColor: C.surface,
    borderRadius: 12,
    borderWidth: 1,
    borderColor: C.line,
    padding: 14,
  },
  date: { fontWeight: "700", color: C.ink, fontSize: 14 },
  meta: { fontSize: 12, color: C.ink3, marginTop: 2 },
  kmTxt: { fontWeight: "800", color: C.ink, fontSize: 14, fontVariant: ["tabular-nums"] },
  empty: { padding: 24, color: C.ink3, fontSize: 13, textAlign: "center" },
});
