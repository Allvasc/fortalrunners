import { useMemo, useState } from "react";
import { ActivityIndicator, FlatList, StyleSheet, Text, TouchableOpacity, View } from "react-native";
import { useQuery } from "@tanstack/react-query";
import { api, type LeaderEntry } from "../lib/api";
import { C } from "../theme";
import { area } from "../format";

// Ranking por área total coberta — território é permanente, sem reset (plano §2).
type Tab = "global" | "friends" | "bairro";

export function RankingScreen() {
  const [tab, setTab] = useState<Tab>("global");
  const cov = useQuery({ queryKey: ["coverage"], queryFn: api.coverage });

  const bairros = useMemo(
    () => (cov.data?.neighborhoods ?? []).slice().sort((a, b) => b.pct - a.pct),
    [cov.data],
  );
  const [nb, setNb] = useState<string | null>(null);
  const activeNb = nb ?? bairros[0]?.neighborhood_id ?? null;

  const board = useQuery({
    queryKey: ["leaderboard", tab, activeNb],
    queryFn: () =>
      tab === "global"
        ? api.leaderboardGlobal()
        : tab === "friends"
          ? api.leaderboardFriends()
          : api.leaderboardNeighborhood(activeNb ?? ""),
    enabled: tab !== "bairro" || !!activeNb,
  });

  const city = cov.data?.city;
  const entries = board.data?.entries ?? [];
  const me = board.data?.me ?? null;
  const meInList = !!me && entries.some((e) => e.user_id === me.user_id);

  return (
    <View style={s.wrap}>
      <View style={s.head}>
        <Text style={s.title}>Ranking</Text>
        <Text style={s.sub}>
          Por área total coberta. Território é permanente — a posição reflete tudo que você já
          conquistou.
        </Text>
        {city && (
          <Text style={s.city}>
            Fortaleza: <Text style={{ color: C.teal, fontWeight: "800" }}>{city.pct}%</Text> ·{" "}
            {city.covered_cells.toLocaleString("pt-BR")} de {city.total_cells.toLocaleString("pt-BR")}{" "}
            células
          </Text>
        )}
        <View style={s.tabs}>
          {(["global", "friends", "bairro"] as Tab[]).map((t) => (
            <TouchableOpacity key={t} style={[s.tab, tab === t && s.tabOn]} onPress={() => setTab(t)}>
              <Text style={[s.tabTxt, tab === t && s.tabTxtOn]}>
                {t === "global" ? "Fortaleza" : t === "friends" ? "Amigos" : "Por bairro"}
              </Text>
            </TouchableOpacity>
          ))}
        </View>

        {tab === "bairro" && bairros.length > 0 && (
          <FlatList
            horizontal
            showsHorizontalScrollIndicator={false}
            data={bairros}
            keyExtractor={(b) => b.neighborhood_id}
            contentContainerStyle={{ gap: 6, paddingVertical: 8 }}
            renderItem={({ item }) => (
              <TouchableOpacity
                style={[s.chip, activeNb === item.neighborhood_id && s.chipOn]}
                onPress={() => setNb(item.neighborhood_id)}
              >
                <Text style={[s.chipTxt, activeNb === item.neighborhood_id && s.chipTxtOn]}>
                  {item.name} · {item.pct}%
                </Text>
              </TouchableOpacity>
            )}
          />
        )}
      </View>

      {board.isLoading && <ActivityIndicator style={{ margin: 24 }} color={C.teal} />}
      {board.isError && <Text style={s.empty}>Não foi possível carregar o ranking.</Text>}

      <FlatList
        data={entries}
        keyExtractor={(e) => e.user_id}
        contentContainerStyle={s.list}
        ListEmptyComponent={
          !board.isLoading && !board.isError ? (
            <Text style={s.empty}>Ninguém conquistou território ainda. Seja o primeiro.</Text>
          ) : null
        }
        renderItem={({ item }) => <Row e={item} me={me?.user_id === item.user_id} />}
        ListFooterComponent={
          me && !meInList ? (
            <>
              <Text style={s.gap}>· · ·</Text>
              <Row e={me} me />
            </>
          ) : null
        }
      />
    </View>
  );
}

function Row({ e, me }: { e: LeaderEntry; me: boolean }) {
  return (
    <View style={[s.row, me && s.rowMe]}>
      <Text style={[s.rank, e.rank <= 3 && s.rankTop]}>{e.rank}º</Text>
      <View style={{ flex: 1, minWidth: 0 }}>
        <Text style={s.name} numberOfLines={1}>
          {e.username} {me && <Text style={s.you}>· você</Text>}
        </Text>
        <Text style={s.meta}>
          {e.athlete_id} · {e.blocks} quart.
        </Text>
      </View>
      <Text style={s.areaTxt}>{area(e.area_m2)}</Text>
    </View>
  );
}

const s = StyleSheet.create({
  wrap: { flex: 1, backgroundColor: C.bg },
  head: { padding: 16, borderBottomWidth: 1, borderBottomColor: C.line },
  title: { fontSize: 20, fontWeight: "800", color: C.ink },
  sub: { fontSize: 13, color: C.ink3, marginTop: 2, lineHeight: 18 },
  city: { fontSize: 12, color: C.ink2, marginTop: 8 },
  tabs: { flexDirection: "row", gap: 6, marginTop: 12 },
  tab: { flex: 1, paddingVertical: 8, borderRadius: 8, backgroundColor: C.sunken, alignItems: "center" },
  tabOn: { backgroundColor: C.teal },
  tabTxt: { fontSize: 12, fontWeight: "700", color: C.ink2 },
  tabTxtOn: { color: "#fff" },
  chip: { paddingHorizontal: 10, paddingVertical: 6, borderRadius: 14, backgroundColor: C.sunken },
  chipOn: { backgroundColor: C.tealDeep },
  chipTxt: { fontSize: 12, color: C.ink2, fontWeight: "600" },
  chipTxtOn: { color: "#fff" },
  list: { padding: 16, gap: 8, paddingBottom: 40 },
  row: {
    flexDirection: "row",
    alignItems: "center",
    gap: 12,
    backgroundColor: C.surface,
    borderRadius: 12,
    borderWidth: 1,
    borderColor: C.line,
    padding: 12,
  },
  rowMe: { borderColor: C.teal, borderWidth: 2 },
  rank: { fontSize: 14, fontWeight: "800", color: C.ink3, width: 34, fontVariant: ["tabular-nums"] },
  rankTop: { color: C.coral },
  name: { fontSize: 14, fontWeight: "700", color: C.ink },
  you: { color: C.teal, fontWeight: "700" },
  meta: { fontSize: 11, color: C.ink3, marginTop: 1 },
  areaTxt: { fontSize: 13, fontWeight: "800", color: C.ink, fontVariant: ["tabular-nums"] },
  empty: { padding: 24, color: C.ink3, fontSize: 13, textAlign: "center" },
  gap: { textAlign: "center", color: C.ink3, marginVertical: 6 },
});
