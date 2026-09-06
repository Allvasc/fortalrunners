import { ScrollView, StyleSheet, Text, View } from "react-native";
import { Ionicons } from "@expo/vector-icons";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import type { NativeStackScreenProps } from "@react-navigation/native-stack";
import { api, type PublicUser } from "../lib/api";
import { C } from "../theme";
import { area, clock, km, pace } from "../format";
import { AuroraBg } from "../ui/AuroraBg";
import { FadeIn } from "../ui/Motion";
import { Tap } from "../ui/Tap";
import type { RootStack } from "../../App";

type Props = NativeStackScreenProps<RootStack, "Home">;

const LINKS: { to: keyof RootStack; label: string; icon: keyof typeof Ionicons.glyphMap }[] = [
  { to: "Landmarks", label: "Marcos", icon: "ribbon-outline" },
  { to: "Routes", label: "Rotas", icon: "trail-sign-outline" },
  { to: "Social", label: "Comunidade", icon: "people-outline" },
  { to: "Clubs", label: "Clubes", icon: "shirt-outline" },
  { to: "Events", label: "Eventos & QR", icon: "qr-code-outline" },
  { to: "AICoach", label: "Coach IA", icon: "sparkles-outline" },
  { to: "Safety", label: "Segurança", icon: "shield-checkmark-outline" },
  { to: "Settings", label: "Ajustes", icon: "settings-outline" },
];

export function Home({ navigation }: Props) {
  const qc = useQueryClient();
  const me = useQuery<PublicUser>({ queryKey: ["me"], queryFn: api.me });
  const life = useQuery({ queryKey: ["lifetime"], queryFn: api.lifetime });
  const cov = useQuery({ queryKey: ["coverage"], queryFn: api.coverage });
  const runs = useQuery({ queryKey: ["runs"], queryFn: () => api.runs(6) });
  const weather = useQuery({ queryKey: ["weather"], queryFn: api.weather });

  return (
    <View style={{ flex: 1, backgroundColor: C.bg }}>
      <AuroraBg tint="teal" />
      <ScrollView contentContainerStyle={s.wrap} showsVerticalScrollIndicator={false}>
        <FadeIn style={s.head}>
          <View>
            <Text style={s.hello}>Olá, {me.data?.username ?? "corredor"}</Text>
            <Text style={s.muted}>{me.data?.athlete_id}</Text>
          </View>
          <Tap
            style={s.logout}
            onPress={async () => {
              try {
                await api.logout();
              } catch {
                /* segue mesmo se o servidor falhar — o token local já foi apagado */
              }
              qc.clear();
              qc.setQueryData(["authed"], false); // força a volta pra tela de login
            }}
          >
            <Ionicons name="log-out-outline" size={20} color={C.ink3} />
          </Tap>
        </FadeIn>

        <FadeIn delay={60}>
          <Tap style={s.tiles} onPress={() => navigation.navigate("Profile")}>
            <Tile v={String(life.data?.run_count ?? 0)} l="corridas" />
            <Tile v={km(life.data?.total_distance_m)} l="km totais" />
            <Tile v={area(life.data?.territory_area_m2)} l="território" />
            <Tile v={`${cov.data?.city.pct ?? 0}%`} l="de Fortaleza" />
          </Tap>
        </FadeIn>

        {weather.data && (
          <FadeIn delay={100}>
            <View style={s.weather}>
              <View style={{ flex: 1 }}>
                <Text style={s.wTemp}>
                  {weather.data.temp_c}°C <Text style={s.wFeels}>· sensação {weather.data.feels_like_c}°</Text>
                </Text>
                <Text style={s.wAdvice} numberOfLines={2}>
                  {weather.data.advice}
                </Text>
              </View>
              <View style={s.wRight}>
                <Text style={s.wUv}>UV {weather.data.uv_index}</Text>
                <Text style={s.wWind}>{weather.data.wind_kmh} km/h</Text>
              </View>
            </View>
          </FadeIn>
        )}

        <FadeIn delay={120}>
          <Tap style={s.cta} to={0.97} onPress={() => navigation.navigate("Recording")}>
            <Ionicons name="play" size={18} color="#fff" />
            <Text style={s.ctaText}>Iniciar corrida</Text>
          </Tap>
        </FadeIn>

        <FadeIn delay={180}>
          <View style={s.grid}>
            {LINKS.map((l) => (
              <Tap key={l.to} style={s.link} onPress={() => navigation.navigate(l.to as never)}>
                <Ionicons name={l.icon} size={20} color={C.teal} />
                <Text style={s.linkTxt}>{l.label}</Text>
              </Tap>
            ))}
          </View>
        </FadeIn>

        <FadeIn delay={240}>
          <Tap style={s.sectionRow} onPress={() => navigation.navigate("History")}>
            <Text style={s.section}>Últimas corridas</Text>
            <Text style={s.seeAll}>ver todas ›</Text>
          </Tap>
        </FadeIn>

        {runs.data?.runs.length === 0 && <Text style={s.muted}>Nenhuma ainda. Bora?</Text>}
        {(runs.data?.runs ?? []).map((r, i) => (
          <FadeIn key={r.id} delay={280 + i * 40}>
            <Tap style={s.runRow} onPress={() => navigation.navigate("Summary", { runId: r.id })}>
              <View>
                <Text style={s.runDate}>
                  {new Date(r.started_at).toLocaleDateString("pt-BR", { day: "2-digit", month: "short" })}
                </Text>
                <Text style={s.muted}>
                  {r.status === "flagged"
                    ? "em revisão"
                    : r.status === "processing"
                      ? "processando…"
                      : `${area(r.territory_area_m2)} · ${r.new_blocks} quart.`}
                </Text>
              </View>
              <View style={{ alignItems: "flex-end" }}>
                <Text style={s.runKm}>{km(r.distance_m)} km</Text>
                <Text style={s.muted}>
                  {pace(r.avg_pace_s)}/km · {clock(r.moving_s)}
                </Text>
              </View>
            </Tap>
          </FadeIn>
        ))}
      </ScrollView>
    </View>
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
  logout: { padding: 6 },
  tiles: { flexDirection: "row", flexWrap: "wrap", gap: 10 },
  tile: {
    flexGrow: 1,
    minWidth: "45%",
    backgroundColor: C.surface,
    borderRadius: 14,
    borderWidth: 1,
    borderColor: C.line,
    padding: 12,
  },
  tileV: { fontSize: 18, fontWeight: "800", color: C.ink },
  tileL: { fontSize: 11, textTransform: "uppercase", color: C.ink3, marginTop: 2 },
  cta: {
    backgroundColor: C.coral,
    borderRadius: 16,
    paddingVertical: 17,
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "center",
    gap: 8,
    shadowColor: C.coral,
    shadowOpacity: 0.28,
    shadowRadius: 14,
    shadowOffset: { width: 0, height: 6 },
    elevation: 5,
  },
  ctaText: { color: "#fff", fontWeight: "800", fontSize: 17 },
  weather: {
    flexDirection: "row",
    gap: 12,
    backgroundColor: C.surface,
    borderWidth: 1,
    borderColor: C.line,
    borderRadius: 14,
    padding: 14,
  },
  wTemp: { fontSize: 16, fontWeight: "800", color: C.ink },
  wFeels: { fontSize: 12, fontWeight: "600", color: C.ink3 },
  wAdvice: { fontSize: 12, color: C.ink2, marginTop: 3, lineHeight: 16 },
  wRight: { alignItems: "flex-end", justifyContent: "center" },
  wUv: { fontSize: 12, fontWeight: "700", color: C.gold },
  wWind: { fontSize: 11, color: C.ink3, marginTop: 2 },
  grid: { flexDirection: "row", flexWrap: "wrap", gap: 10 },
  link: {
    width: "48%",
    flexGrow: 1,
    minWidth: 150,
    backgroundColor: C.surface,
    borderWidth: 1,
    borderColor: C.line,
    borderRadius: 14,
    paddingVertical: 14,
    paddingHorizontal: 12,
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
  },
  linkTxt: { color: C.ink, fontWeight: "700", fontSize: 13, flexShrink: 1, flex: 1 },
  sectionRow: { flexDirection: "row", justifyContent: "space-between", alignItems: "center", marginTop: 6 },
  section: { fontWeight: "800", color: C.ink, fontSize: 16 },
  seeAll: { color: C.teal, fontWeight: "700", fontSize: 13 },
  runRow: {
    flexDirection: "row",
    justifyContent: "space-between",
    backgroundColor: C.surface,
    borderRadius: 12,
    borderWidth: 1,
    borderColor: C.line,
    padding: 14,
  },
  runDate: { fontWeight: "700", color: C.ink },
  runKm: { fontWeight: "700", color: C.ink, fontVariant: ["tabular-nums"] },
});
