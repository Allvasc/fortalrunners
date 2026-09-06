import { FlatList, StyleSheet, Text, View } from "react-native";
import { useQuery } from "@tanstack/react-query";
import { api } from "../lib/api";
import { C } from "../theme";
import { km } from "../format";

export function RoutesScreen() {
  const routesQuery = useQuery({ queryKey: ["routes"], queryFn: api.routes });

  return (
    <View style={s.wrap}>
      <View style={s.head}>
        <Text style={s.title}>Rotas Recomendadas</Text>
        <Text style={s.subtitle}>Percursos curados para correr em Fortaleza</Text>
      </View>

      <FlatList
        data={routesQuery.data?.routes ?? []}
        keyExtractor={(item) => item.id}
        contentContainerStyle={s.list}
        renderItem={({ item }) => (
          <View style={s.card}>
            <View style={s.cardHeader}>
              <Text style={s.cardName}>{item.name}</Text>
              {item.is_official && <Text style={s.officialBadge}>Oficial</Text>}
            </View>

            {item.description && <Text style={s.desc}>{item.description}</Text>}

            <View style={s.cardFooter}>
              <Text style={s.kmText}>{km(item.distance_m)} km · {item.surface}</Text>
              <Text style={s.ratingText}>⭐ {item.avg_rating > 0 ? item.avg_rating.toFixed(1) : "Nova"}</Text>
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
  cardName: { fontSize: 16, fontWeight: "700", color: C.ink, flex: 1 },
  officialBadge: { backgroundColor: C.teal, color: "#fff", fontSize: 10, fontWeight: "800", paddingHorizontal: 6, paddingVertical: 2, borderRadius: 10 },
  desc: { fontSize: 13, color: C.ink2, marginTop: 6, lineHeight: 18 },
  cardFooter: { flexDirection: "row", justifyContent: "space-between", alignItems: "center", marginTop: 12, paddingTop: 8, borderTopWidth: 1, borderTopColor: C.line },
  kmText: { fontSize: 13, fontWeight: "700", color: C.teal },
  ratingText: { fontSize: 12, color: C.ink3, fontWeight: "600" },
});
