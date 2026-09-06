import { useState } from "react";
import {
  ActivityIndicator,
  Alert,
  FlatList,
  Modal,
  ScrollView,
  StyleSheet,
  Text,
  TextInput,
  TouchableOpacity,
  View,
} from "react-native";
import { useQuery } from "@tanstack/react-query";
import { api } from "../lib/api";
import { C } from "../theme";
import { km } from "../format";

export function RoutesScreen() {
  const list = useQuery({ queryKey: ["routes"], queryFn: api.routes });
  const [openId, setOpenId] = useState<string | null>(null);

  return (
    <View style={s.wrap}>
      <View style={s.head}>
        <Text style={s.title}>Rotas Recomendadas</Text>
        <Text style={s.subtitle}>Percursos curados e avaliados pela comunidade</Text>
      </View>

      <FlatList
        data={list.data?.routes ?? []}
        keyExtractor={(r) => r.id}
        contentContainerStyle={s.list}
        ListEmptyComponent={
          list.isLoading ? <ActivityIndicator color={C.teal} style={{ marginTop: 24 }} /> : (
            <Text style={s.empty}>Nenhuma rota cadastrada ainda.</Text>
          )
        }
        renderItem={({ item }) => (
          <TouchableOpacity style={s.card} onPress={() => setOpenId(item.id)}>
            <View style={s.cardHeader}>
              <Text style={s.cardName}>{item.name}</Text>
              {item.is_official && <Text style={s.officialBadge}>Oficial</Text>}
            </View>
            {item.description ? <Text style={s.desc}>{item.description}</Text> : null}
            <View style={s.cardFooter}>
              <Text style={s.kmText}>
                {km(item.distance_m)} km · {item.surface}
              </Text>
              <Text style={s.ratingText}>
                ⭐ {item.avg_rating > 0 ? item.avg_rating.toFixed(1) : "nova"}
                {item.review_count ? ` (${item.review_count})` : ""}
              </Text>
            </View>
          </TouchableOpacity>
        )}
      />

      <RouteDetail id={openId} onClose={() => setOpenId(null)} onReviewed={() => list.refetch()} />
    </View>
  );
}

function RouteDetail({
  id,
  onClose,
  onReviewed,
}: {
  id: string | null;
  onClose: () => void;
  onReviewed: () => void;
}) {
  const detail = useQuery({ queryKey: ["route", id], queryFn: () => api.route(id as string), enabled: !!id });
  const [rating, setRating] = useState(0);
  const [body, setBody] = useState("");
  const [busy, setBusy] = useState(false);

  const submit = async () => {
    if (!id || rating < 1) {
      Alert.alert("Dê uma nota", "Toque nas estrelas para avaliar.");
      return;
    }
    setBusy(true);
    try {
      await api.addRouteReview(id, rating, body.trim());
      Alert.alert("Avaliação enviada", "Obrigado! Ela entra em moderação.");
      setRating(0);
      setBody("");
      detail.refetch();
      onReviewed();
    } catch (e: any) {
      Alert.alert("Ops", e.message || "Erro ao enviar avaliação.");
    } finally {
      setBusy(false);
    }
  };

  const r = detail.data?.route;
  const reviews = detail.data?.reviews ?? [];

  return (
    <Modal visible={!!id} animationType="slide" onRequestClose={onClose}>
      <View style={s.modalWrap}>
        <View style={s.modalHead}>
          <Text style={s.modalTitle} numberOfLines={1}>
            {r?.name ?? "Rota"}
          </Text>
          <TouchableOpacity onPress={onClose} hitSlop={12}>
            <Text style={s.close}>✕</Text>
          </TouchableOpacity>
        </View>

        {detail.isLoading ? (
          <ActivityIndicator color={C.teal} style={{ marginTop: 40 }} />
        ) : (
          <ScrollView contentContainerStyle={{ padding: 16, gap: 14, paddingBottom: 40 }}>
            {r && (
              <View>
                <Text style={s.meta}>
                  {km(r.distance_m)} km · {r.surface} · ⭐ {r.avg_rating > 0 ? r.avg_rating.toFixed(1) : "sem nota"}
                </Text>
                {r.description ? <Text style={s.body}>{r.description}</Text> : null}
              </View>
            )}

            <View style={s.reviewBox}>
              <Text style={s.sectionTitle}>Sua avaliação</Text>
              <View style={{ flexDirection: "row", gap: 4, marginVertical: 8 }}>
                {[1, 2, 3, 4, 5].map((n) => (
                  <TouchableOpacity key={n} onPress={() => setRating(n)}>
                    <Text style={[s.star, n <= rating && s.starOn]}>★</Text>
                  </TouchableOpacity>
                ))}
              </View>
              <TextInput
                style={s.input}
                placeholder="Comentário (opcional): sombra, segurança, piso…"
                placeholderTextColor={C.ink3}
                multiline
                value={body}
                onChangeText={setBody}
              />
              <TouchableOpacity style={[s.submit, busy && { opacity: 0.5 }]} disabled={busy} onPress={submit}>
                <Text style={s.submitTxt}>{busy ? "Enviando…" : "Enviar avaliação"}</Text>
              </TouchableOpacity>
            </View>

            <Text style={s.sectionTitle}>Avaliações ({reviews.length})</Text>
            {reviews.length === 0 && <Text style={s.emptySm}>Ainda sem avaliações aprovadas.</Text>}
            {reviews.map((rv) => (
              <View key={rv.id} style={s.reviewCard}>
                <View style={{ flexDirection: "row", justifyContent: "space-between" }}>
                  <Text style={s.reviewer}>{rv.user_name || "corredor"}</Text>
                  <Text style={s.starOn}>{"★".repeat(rv.rating)}</Text>
                </View>
                {rv.body ? <Text style={s.body}>{rv.body}</Text> : null}
              </View>
            ))}
          </ScrollView>
        )}
      </View>
    </Modal>
  );
}

const s = StyleSheet.create({
  wrap: { flex: 1, backgroundColor: C.bg },
  head: { padding: 16, borderBottomWidth: 1, borderBottomColor: C.line },
  title: { fontSize: 20, fontWeight: "800", color: C.ink },
  subtitle: { fontSize: 13, color: C.ink3, marginTop: 2 },
  list: { padding: 16, gap: 12, paddingBottom: 40 },
  card: { backgroundColor: C.surface, borderRadius: 12, borderWidth: 1, borderColor: C.line, padding: 14 },
  cardHeader: { flexDirection: "row", justifyContent: "space-between", alignItems: "center", gap: 8 },
  cardName: { fontSize: 16, fontWeight: "700", color: C.ink, flex: 1 },
  officialBadge: {
    backgroundColor: C.teal,
    color: "#fff",
    fontSize: 10,
    fontWeight: "800",
    paddingHorizontal: 6,
    paddingVertical: 2,
    borderRadius: 10,
    overflow: "hidden",
  },
  desc: { fontSize: 13, color: C.ink2, marginTop: 6, lineHeight: 18 },
  cardFooter: {
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
    marginTop: 12,
    paddingTop: 8,
    borderTopWidth: 1,
    borderTopColor: C.line,
  },
  kmText: { fontSize: 13, fontWeight: "700", color: C.teal },
  ratingText: { fontSize: 12, color: C.ink3, fontWeight: "600" },
  empty: { padding: 24, color: C.ink3, fontSize: 13, textAlign: "center" },
  emptySm: { color: C.ink3, fontSize: 12 },
  modalWrap: { flex: 1, backgroundColor: C.bg },
  modalHead: {
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
    padding: 16,
    borderBottomWidth: 1,
    borderBottomColor: C.line,
  },
  modalTitle: { fontSize: 17, fontWeight: "800", color: C.ink, flex: 1 },
  close: { fontSize: 18, color: C.ink3 },
  meta: { fontSize: 13, color: C.ink2, fontWeight: "600" },
  body: { fontSize: 13, color: C.ink2, marginTop: 6, lineHeight: 19 },
  sectionTitle: { fontSize: 14, fontWeight: "800", color: C.ink },
  reviewBox: { backgroundColor: C.surface, borderRadius: 12, borderWidth: 1, borderColor: C.line, padding: 14 },
  star: { fontSize: 28, color: C.line },
  starOn: { color: C.coral },
  input: {
    borderWidth: 1,
    borderColor: C.line,
    borderRadius: 8,
    padding: 10,
    minHeight: 60,
    textAlignVertical: "top",
    color: C.ink,
    backgroundColor: C.bg,
    marginBottom: 10,
  },
  submit: { backgroundColor: C.teal, borderRadius: 8, paddingVertical: 10, alignItems: "center" },
  submitTxt: { color: "#fff", fontWeight: "700" },
  reviewCard: { backgroundColor: C.surface, borderRadius: 10, borderWidth: 1, borderColor: C.line, padding: 12 },
  reviewer: { fontSize: 13, fontWeight: "700", color: C.ink },
});
