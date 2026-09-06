import { FlatList, StyleSheet, Text, TouchableOpacity, View } from "react-native";
import { useQuery } from "@tanstack/react-query";
import { api } from "../lib/api";
import { C } from "../theme";

export function SocialScreen() {
  const feedQuery = useQuery({ queryKey: ["feed"], queryFn: api.feed });

  const handleKudos = async (runId: string) => {
    try {
      await api.toggleKudos(runId);
      feedQuery.refetch();
    } catch {}
  };

  return (
    <View style={s.wrap}>
      <View style={s.head}>
        <Text style={s.title}>Feed da Comunidade</Text>
        <Text style={s.subtitle}>Atividades recentes dos corredores em Fortaleza</Text>
      </View>

      <FlatList
        data={feedQuery.data?.feed ?? []}
        keyExtractor={(item) => item.id}
        contentContainerStyle={s.list}
        renderItem={({ item }) => (
          <View style={s.card}>
            <View style={s.cardHeader}>
              <Text style={s.actorName}>{item.actor_name}</Text>
              <Text style={s.dateText}>{new Date(item.created_at).toLocaleDateString()}</Text>
            </View>

            <Text style={s.eventDesc}>
              {item.type === "run" && "🏃 completou uma corrida na cidade!"}
              {item.type === "claim" && "🚩 conquistou um novo território em Fortaleza!"}
              {item.type === "badge" && "🏆 desbloqueou um novo selo histórico!"}
              {item.type === "checkin" && "📍 fez check-in em um marco turístico!"}
            </Text>

            {item.type === "run" && (
              <View style={s.cardFooter}>
                <TouchableOpacity
                  style={[s.btnKudos, item.has_kudos ? s.btnKudosActive : s.btnKudosInactive]}
                  onPress={() => handleKudos(item.subject_id)}
                >
                  <Text style={[s.kudosText, item.has_kudos ? s.kudosTextActive : s.kudosTextInactive]}>
                    👏 Kudos ({item.kudos_count})
                  </Text>
                </TouchableOpacity>
              </View>
            )}
          </View>
        )}
      />
    </View>
  );
}

const s = StyleSheet.create({
  wrap: { flex: 1, backgroundColor: C.bg },
  head: { padding: 16, borderBottomWidth: 1, borderBottomColor: C.line },
  title: { fontSize: 20, fontWeight: "800", color: C.ink },
  subtitle: { fontSize: 13, color: C.ink3, marginTop: 2 },
  list: { padding: 16, gap: 12 },
  card: { backgroundColor: C.surface, borderRadius: 12, borderWidth: 1, borderColor: C.line, padding: 14 },
  cardHeader: { flexDirection: "row", justifyContent: "space-between", alignItems: "center" },
  actorName: { fontSize: 15, fontWeight: "700", color: C.ink },
  dateText: { fontSize: 12, color: C.ink3 },
  eventDesc: { fontSize: 13, color: C.ink2, marginTop: 6, lineHeight: 18 },
  cardFooter: { marginTop: 10, paddingTop: 8, borderTopWidth: 1, borderTopColor: C.line },
  btnKudos: { paddingHorizontal: 12, paddingVertical: 6, borderRadius: 16, alignSelf: "flex-start" },
  btnKudosActive: { backgroundColor: C.coral },
  btnKudosInactive: { backgroundColor: C.line },
  kudosText: { fontSize: 12, fontWeight: "700" },
  kudosTextActive: { color: "#fff" },
  kudosTextInactive: { color: C.ink },
});
