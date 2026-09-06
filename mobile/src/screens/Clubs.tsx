import { Alert, FlatList, StyleSheet, Text, TouchableOpacity, View } from "react-native";
import { useQuery } from "@tanstack/react-query";
import { api } from "../lib/api";
import { C } from "../theme";

export function ClubsScreen() {
  const clubsQuery = useQuery({ queryKey: ["clubs"], queryFn: api.clubs });

  const handleJoin = async (id: string) => {
    try {
      await api.joinClub(id);
      Alert.alert("Sucesso! 🎉", "Você agora faz parte deste clube!");
      clubsQuery.refetch();
    } catch (err: any) {
      Alert.alert("Ops", err.message || "Erro ao entrar no clube.");
    }
  };

  return (
    <View style={s.wrap}>
      <View style={s.head}>
        <Text style={s.title}>Clubes de Corrida</Text>
        <Text style={s.subtitle}>Grupos e equipes em Fortaleza</Text>
      </View>

      <FlatList
        data={clubsQuery.data?.clubs ?? []}
        keyExtractor={(item) => item.id}
        contentContainerStyle={s.list}
        renderItem={({ item }) => (
          <View style={[s.card, { borderLeftColor: item.color_hex, borderLeftWidth: 5 }]}>
            <View style={s.cardHeader}>
              <Text style={s.clubName}>{item.name}</Text>
              <Text style={s.membersBadge}>{item.member_count} membros</Text>
            </View>

            {item.description && <Text style={s.desc}>{item.description}</Text>}

            <View style={s.cardFooter}>
              <Text style={s.statusText}>{item.is_member ? "✓ Você é membro" : "Aberto"}</Text>
              {!item.is_member && (
                <TouchableOpacity style={s.btnJoin} onPress={() => handleJoin(item.id)}>
                  <Text style={s.btnText}>Entrar no Clube</Text>
                </TouchableOpacity>
              )}
            </View>
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
  clubName: { fontSize: 16, fontWeight: "700", color: C.ink, flex: 1 },
  membersBadge: { fontSize: 11, color: C.ink3, backgroundColor: C.line, paddingHorizontal: 8, paddingVertical: 2, borderRadius: 10 },
  desc: { fontSize: 13, color: C.ink2, marginTop: 6, lineHeight: 18 },
  cardFooter: { flexDirection: "row", justifyContent: "space-between", alignItems: "center", marginTop: 12, paddingTop: 8, borderTopWidth: 1, borderTopColor: C.line },
  statusText: { fontSize: 12, color: C.ink3 },
  btnJoin: { backgroundColor: C.teal, borderRadius: 6, paddingHorizontal: 12, paddingVertical: 6 },
  btnText: { color: "#fff", fontWeight: "700", fontSize: 12 },
});
