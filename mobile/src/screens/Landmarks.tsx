import { useState } from "react";
import { Alert, FlatList, StyleSheet, Text, TouchableOpacity, View } from "react-native";
import { useQuery } from "@tanstack/react-query";
import * as Location from "expo-location";
import { api } from "../lib/api";
import { C } from "../theme";

export function Landmarks() {
  const lmkQuery = useQuery({ queryKey: ["landmarks"], queryFn: api.landmarksProgress });
  const [checkingId, setCheckingId] = useState<string | null>(null);

  const handleCheckin = async (id: string, name: string) => {
    setCheckingId(id);
    try {
      const { status } = await Location.requestForegroundPermissionsAsync();
      if (status !== "granted") {
        Alert.alert("Permissão necessária", "É necessário permitir o acesso ao GPS para realizar check-in.");
        return;
      }
      const loc = await Location.getCurrentPositionAsync({ accuracy: Location.Accuracy.High });
      await api.landmarkCheckin(id, loc.coords.latitude, loc.coords.longitude);
      Alert.alert("Parabéns! 🎉", `Selo "${name}" conquistado com sucesso!`);
      lmkQuery.refetch();
    } catch (err: any) {
      Alert.alert("Ops", err.message || "Erro ao fazer check-in.");
    } finally {
      setCheckingId(null);
    }
  };

  const data = lmkQuery.data;
  const pct = data && data.total_landmarks > 0 ? Math.round((data.unlocked_count / data.total_landmarks) * 100) : 0;

  return (
    <View style={s.wrap}>
      <View style={s.head}>
        <Text style={s.title}>Marcos Históricos & Selos</Text>
        <Text style={s.subtitle}>Conquiste os pontos turísticos de Fortaleza correndo</Text>

        <View style={s.progressCard}>
          <View style={s.progressRow}>
            <Text style={s.progressText}>Conquistados</Text>
            <Text style={s.progressVal}>{data?.unlocked_count ?? 0} / {data?.total_landmarks ?? 0} ({pct}%)</Text>
          </View>
          <View style={s.progressBarTrack}>
            <View style={[s.progressBarFill, { width: `${pct}%` }]} />
          </View>
        </View>
      </View>

      <FlatList
        data={data?.landmarks ?? []}
        keyExtractor={(item) => item.id}
        contentContainerStyle={s.list}
        renderItem={({ item }) => (
          <View style={s.card}>
            <View style={s.cardHeader}>
              <Text style={s.cardName}>{item.name}</Text>
              <Text style={[s.badgeTag, item.checked_in ? s.checkedInTag : s.lockedTag]}>
                {item.checked_in ? "✓ Conquistado" : "🔒 Bloqueado"}
              </Text>
            </View>

            {item.blurb && <Text style={s.blurb}>{item.blurb}</Text>}

            <View style={s.cardFooter}>
              <Text style={s.radiusText}>Raio: {item.radius_m}m</Text>
              {!item.checked_in && (
                <TouchableOpacity
                  style={s.btnCheckin}
                  disabled={checkingId === item.id}
                  onPress={() => handleCheckin(item.id, item.name)}
                >
                  <Text style={s.btnText}>{checkingId === item.id ? "Verificando..." : "Check-in GPS"}</Text>
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
  subtitle: { fontSize: 13, color: C.ink3, marginTop: 2, marginBottom: 12 },
  progressCard: { backgroundColor: C.surface, borderRadius: 10, padding: 12, borderWidth: 1, borderColor: C.line },
  progressRow: { flexDirection: "row", justifyContent: "space-between", marginBottom: 6 },
  progressText: { fontSize: 13, fontWeight: "700", color: C.ink },
  progressVal: { fontSize: 13, fontWeight: "700", color: C.teal },
  progressBarTrack: { height: 6, backgroundColor: C.line, borderRadius: 3, overflow: "hidden" },
  progressBarFill: { height: "100%", backgroundColor: C.teal },
  list: { padding: 16, gap: 12 },
  card: { backgroundColor: C.surface, borderRadius: 12, borderWidth: 1, borderColor: C.line, padding: 14 },
  cardHeader: { flexDirection: "row", justifyContent: "space-between", alignItems: "center" },
  cardName: { fontSize: 16, fontWeight: "700", color: C.ink, flex: 1 },
  badgeTag: { fontSize: 11, fontWeight: "700", paddingHorizontal: 8, paddingVertical: 3, borderRadius: 12 },
  checkedInTag: { backgroundColor: "rgba(46, 143, 99, 0.15)", color: C.good },
  lockedTag: { backgroundColor: C.line, color: C.ink3 },
  blurb: { fontSize: 13, color: C.ink2, marginTop: 6, lineHeight: 18 },
  cardFooter: { flexDirection: "row", justifyContent: "space-between", alignItems: "center", marginTop: 12, paddingTop: 8, borderTopWidth: 1, borderTopColor: C.line },
  radiusText: { fontSize: 12, color: C.ink3 },
  btnCheckin: { backgroundColor: C.teal, borderRadius: 6, paddingHorizontal: 12, paddingVertical: 6 },
  btnText: { color: "#fff", fontWeight: "700", fontSize: 12 },
});
