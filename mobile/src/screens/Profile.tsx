import { ActivityIndicator, ScrollView, StyleSheet, Text, View } from "react-native";
import { useQuery } from "@tanstack/react-query";
import { api, type PublicUser } from "../lib/api";
import { C } from "../theme";
import { area, clock, km } from "../format";
import { AuroraBg } from "../ui/AuroraBg";
import { FadeIn } from "../ui/Motion";

// "Números de sempre" — estatísticas de vida, recordes e selos (plano §4).

const REC_LABEL: Record<string, string> = {
  longest: "Maior distância",
  max_elev: "Mais elevação",
  biggest_territory: "Maior território",
  "1h": "Recorde de 1 hora",
  "1k": "1 km mais rápido",
  "5k": "5 km mais rápido",
  "10k": "10 km mais rápido",
  "15k": "15 km mais rápido",
  "21k": "Meia maratona",
  "42k": "Maratona",
};

function recValue(key: string, v: number): string {
  if (key === "longest" || key === "1h") return `${km(v)} km`;
  if (key === "max_elev") return `${Math.round(v)} m`;
  if (key === "biggest_territory") return area(v);
  return clock(v); // 1k..42k → tempo
}

export function ProfileScreen() {
  const me = useQuery<PublicUser>({ queryKey: ["me"], queryFn: api.me });
  const life = useQuery({ queryKey: ["lifetime"], queryFn: api.lifetime });
  const recs = useQuery({ queryKey: ["records"], queryFn: api.records });
  const badges = useQuery({ queryKey: ["badges"], queryFn: api.badges });

  const l = life.data;

  return (
    <View style={{ flex: 1, backgroundColor: C.bg }}>
      <AuroraBg tint="gold" />
      <ScrollView contentContainerStyle={s.wrap} showsVerticalScrollIndicator={false}>
      <FadeIn>
        <Text style={s.name}>{me.data?.username ?? "corredor"}</Text>
        <Text style={s.muted}>
          {me.data?.athlete_id} · {me.data?.email}
        </Text>
      </FadeIn>

      <Text style={s.section}>Números de sempre</Text>
      <View style={s.tiles}>
        <Tile v={String(l?.run_count ?? 0)} k="corridas" />
        <Tile v={`${km(l?.total_distance_m)} km`} k="distância" />
        <Tile v={clock(l?.total_moving_s)} k="em movimento" />
        <Tile v={area(l?.territory_area_m2)} k="território" />
        <Tile v={`${Math.round(l?.elevation_gain_m ?? 0)} m`} k="elevação" />
        <Tile v={String(l?.total_steps ?? 0)} k="passos" />
        <Tile v={String(l?.active_days ?? 0)} k="dias ativos" />
        <Tile v={`${l?.current_streak_days ?? 0} d`} k="sequência atual" />
        <Tile v={`${l?.longest_streak_days ?? 0} d`} k="maior sequência" />
        <Tile v={l?.last_run_date ?? "—"} k="última corrida" />
      </View>

      <Text style={s.section}>Recordes pessoais</Text>
      {recs.isLoading && <ActivityIndicator color={C.teal} />}
      {recs.data && recs.data.records.length === 0 && (
        <Text style={s.muted}>Ainda sem recordes — eles aparecem depois da primeira corrida válida.</Text>
      )}
      {(recs.data?.records ?? []).map((r) => (
        <View key={r.distance_key} style={s.recRow}>
          <Text style={s.recLabel}>{REC_LABEL[r.distance_key] ?? r.distance_key}</Text>
          <Text style={s.recVal}>{recValue(r.distance_key, r.value_s)}</Text>
        </View>
      ))}

      <Text style={s.section}>Selos</Text>
      {badges.isLoading && <ActivityIndicator color={C.teal} />}
      {badges.data && badges.data.badges.length === 0 && (
        <Text style={s.muted}>Nenhum selo ainda. Conquiste marcos e desafios para colecionar.</Text>
      )}
      <View style={{ gap: 8 }}>
        {(badges.data?.badges ?? []).map((b) => (
          <View key={b.code} style={s.badge}>
            <Text style={s.badgeName}>{b.name}</Text>
            {b.description && <Text style={s.muted}>{b.description}</Text>}
            <Text style={s.badgeDate}>
              {new Date(b.earned_at).toLocaleDateString("pt-BR")}
            </Text>
          </View>
        ))}
      </View>
      </ScrollView>
    </View>
  );
}

function Tile({ v, k }: { v: string; k: string }) {
  return (
    <View style={s.tile}>
      <Text style={s.tileV} numberOfLines={1}>
        {v}
      </Text>
      <Text style={s.tileK}>{k}</Text>
    </View>
  );
}

const s = StyleSheet.create({
  wrap: { padding: 20, gap: 14, paddingBottom: 44 },
  name: { fontSize: 22, fontWeight: "800", color: C.ink },
  muted: { color: C.ink3, fontSize: 13, lineHeight: 18 },
  section: { fontWeight: "800", color: C.ink, fontSize: 16, marginTop: 10 },
  tiles: { flexDirection: "row", flexWrap: "wrap", gap: 10 },
  tile: {
    flexGrow: 1,
    minWidth: "45%",
    backgroundColor: C.surface,
    borderRadius: 12,
    borderWidth: 1,
    borderColor: C.line,
    padding: 12,
  },
  tileV: { fontSize: 17, fontWeight: "800", color: C.ink },
  tileK: { fontSize: 11, textTransform: "uppercase", color: C.ink3, marginTop: 2 },
  recRow: {
    flexDirection: "row",
    justifyContent: "space-between",
    backgroundColor: C.surface,
    borderRadius: 10,
    borderWidth: 1,
    borderColor: C.line,
    padding: 12,
  },
  recLabel: { fontSize: 13, color: C.ink2, fontWeight: "600" },
  recVal: { fontSize: 13, fontWeight: "800", color: C.ink, fontVariant: ["tabular-nums"] },
  badge: { backgroundColor: C.surface, borderRadius: 10, borderWidth: 1, borderColor: C.line, padding: 12 },
  badgeName: { fontSize: 14, fontWeight: "700", color: C.ink },
  badgeDate: { fontSize: 11, color: C.ink3, marginTop: 4 },
});
