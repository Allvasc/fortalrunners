import { useState } from "react";
import {
  ActivityIndicator,
  Alert,
  FlatList,
  StyleSheet,
  Text,
  TextInput,
  TouchableOpacity,
  View,
} from "react-native";
import { useQuery } from "@tanstack/react-query";
import { api, type Friend } from "../lib/api";
import { C } from "../theme";

type Tab = "feed" | "amigos";

export function SocialScreen() {
  const [tab, setTab] = useState<Tab>("feed");
  const feed = useQuery({ queryKey: ["feed"], queryFn: api.feed });
  const friends = useQuery({ queryKey: ["friends"], queryFn: api.friends });
  const [target, setTarget] = useState("");
  const [busy, setBusy] = useState(false);

  const kudos = async (runId: string) => {
    try {
      await api.toggleKudos(runId);
      feed.refetch();
    } catch {}
  };

  const invite = async () => {
    const id = target.trim();
    if (!id) return;
    setBusy(true);
    try {
      await api.sendFriendRequest(id);
      Alert.alert("Convite enviado", `Pedido de amizade enviado para ${id}.`);
      setTarget("");
      friends.refetch();
    } catch (e: any) {
      Alert.alert("Ops", e.message || "Não foi possível enviar o convite.");
    } finally {
      setBusy(false);
    }
  };

  const accept = async (f: Friend) => {
    try {
      await api.acceptFriendRequest(f.id);
      friends.refetch();
    } catch (e: any) {
      Alert.alert("Ops", e.message || "Erro ao aceitar.");
    }
  };

  const remove = async (f: Friend) => {
    try {
      await api.removeFriend(f.id);
      friends.refetch();
    } catch (e: any) {
      Alert.alert("Ops", e.message || "Erro ao remover.");
    }
  };

  return (
    <View style={s.wrap}>
      <View style={s.head}>
        <Text style={s.title}>Comunidade</Text>
        <View style={s.tabs}>
          <Tab label="Feed" on={tab === "feed"} onPress={() => setTab("feed")} />
          <Tab
            label={`Amigos${friends.data ? ` (${friends.data.friends.filter((f) => f.status === "accepted").length})` : ""}`}
            on={tab === "amigos"}
            onPress={() => setTab("amigos")}
          />
        </View>
      </View>

      {tab === "feed" ? (
        <FlatList
          data={feed.data?.feed ?? []}
          keyExtractor={(i) => i.id}
          contentContainerStyle={s.list}
          ListEmptyComponent={
            feed.isLoading ? <ActivityIndicator color={C.teal} style={{ marginTop: 24 }} /> : (
              <Text style={s.empty}>Nada no feed ainda. Adicione amigos ou corra.</Text>
            )
          }
          renderItem={({ item }) => (
            <View style={s.card}>
              <View style={s.cardHead}>
                <Text style={s.actor}>{item.actor_name}</Text>
                <Text style={s.date}>{new Date(item.created_at).toLocaleDateString("pt-BR")}</Text>
              </View>
              <Text style={s.desc}>
                {item.type === "run" && "🏃 completou uma corrida"}
                {item.type === "claim" && "🚩 conquistou novo território"}
                {item.type === "badge" && "🏆 desbloqueou um selo"}
                {item.type === "checkin" && "📍 fez check-in num marco"}
              </Text>
              {item.type === "run" && (
                <TouchableOpacity
                  style={[s.kudos, item.has_kudos ? s.kudosOn : s.kudosOff]}
                  onPress={() => kudos(item.subject_id)}
                >
                  <Text style={[s.kudosTxt, { color: item.has_kudos ? "#fff" : C.ink }]}>
                    👏 {item.kudos_count}
                  </Text>
                </TouchableOpacity>
              )}
            </View>
          )}
        />
      ) : (
        <FlatList
          data={friends.data?.friends ?? []}
          keyExtractor={(f) => f.id}
          contentContainerStyle={s.list}
          ListHeaderComponent={
            <View style={s.invite}>
              <Text style={s.inviteLabel}>Adicionar amigo</Text>
              <View style={{ flexDirection: "row", gap: 8 }}>
                <TextInput
                  style={s.input}
                  placeholder="ID do atleta (FR-0001234)"
                  placeholderTextColor={C.ink3}
                  autoCapitalize="characters"
                  value={target}
                  onChangeText={setTarget}
                />
                <TouchableOpacity style={[s.inviteBtn, busy && { opacity: 0.5 }]} disabled={busy} onPress={invite}>
                  <Text style={s.inviteBtnTxt}>{busy ? "…" : "Convidar"}</Text>
                </TouchableOpacity>
              </View>
            </View>
          }
          ListEmptyComponent={
            friends.isLoading ? <ActivityIndicator color={C.teal} style={{ marginTop: 16 }} /> : (
              <Text style={s.empty}>Nenhum amigo ainda.</Text>
            )
          }
          renderItem={({ item }) => (
            <View style={s.card}>
              <View style={s.cardHead}>
                <View style={{ flex: 1, minWidth: 0 }}>
                  <Text style={s.actor} numberOfLines={1}>
                    {item.name || item.username}
                  </Text>
                  <Text style={s.date}>{item.athlete_id}</Text>
                </View>
                {item.status === "accepted" ? (
                  <TouchableOpacity onPress={() => remove(item)}>
                    <Text style={s.linkMuted}>remover</Text>
                  </TouchableOpacity>
                ) : item.incoming ? (
                  <TouchableOpacity style={s.acceptBtn} onPress={() => accept(item)}>
                    <Text style={s.acceptTxt}>aceitar</Text>
                  </TouchableOpacity>
                ) : (
                  <Text style={s.linkMuted}>pendente</Text>
                )}
              </View>
            </View>
          )}
        />
      )}
    </View>
  );
}

function Tab({ label, on, onPress }: { label: string; on: boolean; onPress: () => void }) {
  return (
    <TouchableOpacity style={[s.tab, on && s.tabOn]} onPress={onPress}>
      <Text style={[s.tabTxt, on && s.tabTxtOn]}>{label}</Text>
    </TouchableOpacity>
  );
}

const s = StyleSheet.create({
  wrap: { flex: 1, backgroundColor: C.bg },
  head: { padding: 16, borderBottomWidth: 1, borderBottomColor: C.line },
  title: { fontSize: 20, fontWeight: "800", color: C.ink },
  tabs: { flexDirection: "row", gap: 6, marginTop: 10 },
  tab: { flex: 1, paddingVertical: 8, borderRadius: 8, backgroundColor: C.sunken, alignItems: "center" },
  tabOn: { backgroundColor: C.teal },
  tabTxt: { fontSize: 12, fontWeight: "700", color: C.ink2 },
  tabTxtOn: { color: "#fff" },
  list: { padding: 16, gap: 10, paddingBottom: 40 },
  card: { backgroundColor: C.surface, borderRadius: 12, borderWidth: 1, borderColor: C.line, padding: 14 },
  cardHead: { flexDirection: "row", justifyContent: "space-between", alignItems: "center", gap: 10 },
  actor: { fontSize: 15, fontWeight: "700", color: C.ink },
  date: { fontSize: 12, color: C.ink3 },
  desc: { fontSize: 13, color: C.ink2, marginTop: 6 },
  kudos: { marginTop: 10, alignSelf: "flex-start", paddingHorizontal: 12, paddingVertical: 6, borderRadius: 16 },
  kudosOn: { backgroundColor: C.coral },
  kudosOff: { backgroundColor: C.line },
  kudosTxt: { fontSize: 12, fontWeight: "700" },
  invite: { backgroundColor: C.surface, borderRadius: 12, borderWidth: 1, borderColor: C.line, padding: 14, marginBottom: 4 },
  inviteLabel: { fontSize: 13, fontWeight: "700", color: C.ink, marginBottom: 8 },
  input: {
    flex: 1,
    borderWidth: 1,
    borderColor: C.line,
    borderRadius: 8,
    paddingHorizontal: 10,
    paddingVertical: 8,
    color: C.ink,
    backgroundColor: C.bg,
  },
  inviteBtn: { backgroundColor: C.teal, borderRadius: 8, paddingHorizontal: 14, justifyContent: "center" },
  inviteBtnTxt: { color: "#fff", fontWeight: "700", fontSize: 13 },
  acceptBtn: { backgroundColor: C.teal, borderRadius: 6, paddingHorizontal: 10, paddingVertical: 5 },
  acceptTxt: { color: "#fff", fontWeight: "700", fontSize: 12 },
  linkMuted: { color: C.ink3, fontSize: 12 },
  empty: { padding: 24, color: C.ink3, fontSize: 13, textAlign: "center" },
});
