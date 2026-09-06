import { ActivityIndicator, FlatList, StyleSheet, Text, View } from "react-native";
import { useQuery } from "@tanstack/react-query";
import { api, type ChallengeView } from "../lib/api";
import { C } from "../theme";
import { area, km } from "../format";

// Desafios: competições com janela fechada — semana, quinzena, mês. Rendem selo
// e posição, não mexem no território (plano §2).

const CADENCE: Record<string, string> = {
  weekly: "Semanal",
  biweekly: "Quinzenal",
  monthly: "Mensal",
  oneoff: "Especial",
};

function fmtMetric(metric: string, value: number): string {
  switch (metric) {
    case "new_area_m2":
      return area(value);
    case "distance_m":
      return `${km(value)} km`;
    case "elevation_gain_m":
      return `${Math.round(value)} m`;
    case "new_blocks":
      return `${Math.round(value)} quart.`;
    case "new_neighborhoods":
      return `${Math.round(value)} bairros`;
    default:
      return String(Math.round(value));
  }
}

function timeLeft(endsAt: string): string {
  const ms = new Date(endsAt).getTime() - Date.now();
  if (ms <= 0) return "encerrado";
  const h = Math.floor(ms / 3_600_000);
  if (h < 24) return `faltam ${h}h`;
  return `faltam ${Math.floor(h / 24)}d`;
}

export function ChallengesScreen() {
  const q = useQuery({ queryKey: ["challenges"], queryFn: api.challenges });

  return (
    <View style={s.wrap}>
      <View style={s.head}>
        <Text style={s.title}>Desafios</Text>
        <Text style={s.sub}>
          Competições com janela fechada. Rendem selo e posição —{" "}
          <Text style={{ fontWeight: "800" }}>não mexem no seu território.</Text>
        </Text>
      </View>

      {q.isLoading && <ActivityIndicator style={{ margin: 24 }} color={C.teal} />}
      {q.isError && <Text style={s.empty}>Não foi possível carregar os desafios.</Text>}

      <FlatList
        data={q.data?.challenges ?? []}
        keyExtractor={(c) => c.slug}
        contentContainerStyle={s.list}
        ListEmptyComponent={
          !q.isLoading && !q.isError ? (
            <Text style={s.empty}>Nenhum desafio ativo no momento.</Text>
          ) : null
        }
        renderItem={({ item }) => <Card c={item} />}
      />
    </View>
  );
}

function Card({ c }: { c: ChallengeView }) {
  const goalPct =
    c.goal && c.goal > 0 ? Math.min(100, Math.round((c.my_value / c.goal) * 100)) : null;

  return (
    <View style={s.card}>
      <View style={s.cardTop}>
        <Text style={s.cadence}>{CADENCE[c.cadence] ?? c.cadence}</Text>
        <Text style={s.left}>{timeLeft(c.ends_at)}</Text>
      </View>
      <Text style={s.name}>{c.title}</Text>
      <Text style={s.desc}>{c.description}</Text>

      <View style={s.stats}>
        <View>
          <Text style={s.statV}>{fmtMetric(c.metric, c.my_value)}</Text>
          <Text style={s.statL}>seu total</Text>
        </View>
        {c.my_rank > 0 && (
          <View style={{ alignItems: "flex-end" }}>
            <Text style={s.statV}>{c.my_rank}º</Text>
            <Text style={s.statL}>sua posição</Text>
          </View>
        )}
      </View>

      {goalPct !== null && (
        <>
          <View style={s.track}>
            <View style={[s.fill, { width: `${goalPct}%` }, c.my_completed && { backgroundColor: C.good }]} />
          </View>
          <Text style={s.goalTxt}>
            {c.my_completed ? "✓ meta batida" : `${goalPct}% da meta (${fmtMetric(c.metric, c.goal ?? 0)})`}
          </Text>
        </>
      )}
    </View>
  );
}

const s = StyleSheet.create({
  wrap: { flex: 1, backgroundColor: C.bg },
  head: { padding: 16, borderBottomWidth: 1, borderBottomColor: C.line },
  title: { fontSize: 20, fontWeight: "800", color: C.ink },
  sub: { fontSize: 13, color: C.ink3, marginTop: 2, lineHeight: 18 },
  list: { padding: 16, gap: 12, paddingBottom: 40 },
  card: { backgroundColor: C.surface, borderRadius: 12, borderWidth: 1, borderColor: C.line, padding: 14 },
  cardTop: { flexDirection: "row", justifyContent: "space-between", alignItems: "center" },
  cadence: {
    fontSize: 10,
    fontWeight: "800",
    letterSpacing: 0.5,
    textTransform: "uppercase",
    color: C.ink2,
    backgroundColor: C.sunken,
    paddingHorizontal: 6,
    paddingVertical: 2,
    borderRadius: 4,
    overflow: "hidden",
  },
  left: { fontSize: 12, color: C.ink3 },
  name: { fontSize: 16, fontWeight: "800", color: C.ink, marginTop: 8 },
  desc: { fontSize: 13, color: C.ink2, marginTop: 3, lineHeight: 18 },
  stats: { flexDirection: "row", justifyContent: "space-between", marginTop: 12 },
  statV: { fontSize: 17, fontWeight: "800", color: C.ink },
  statL: { fontSize: 11, color: C.ink3, marginTop: 1 },
  track: { height: 6, backgroundColor: C.line, borderRadius: 3, overflow: "hidden", marginTop: 12 },
  fill: { height: "100%", backgroundColor: C.teal },
  goalTxt: { fontSize: 11, color: C.ink3, marginTop: 5 },
  empty: { padding: 24, color: C.ink3, fontSize: 13, textAlign: "center" },
});
