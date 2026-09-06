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
import { api } from "../lib/api";
import { C } from "../theme";

const COLORS = ["#0E7C86", "#2E8F63", "#E0562F", "#6E5AA6", "#C98F2E", "#C7402B"];

export function ClubsScreen() {
  const clubs = useQuery({ queryKey: ["clubs"], queryFn: api.clubs });
  const [showForm, setShowForm] = useState(false);
  const [name, setName] = useState("");
  const [desc, setDesc] = useState("");
  const [color, setColor] = useState(COLORS[0]);
  const [busy, setBusy] = useState(false);

  const join = async (id: string) => {
    try {
      await api.joinClub(id);
      clubs.refetch();
    } catch (e: any) {
      Alert.alert("Ops", e.message || "Erro ao entrar no clube.");
    }
  };

  const create = async () => {
    if (!name.trim()) return;
    setBusy(true);
    try {
      await api.createClub(name.trim(), desc.trim(), color);
      Alert.alert("Clube criado! 🎉");
      setName("");
      setDesc("");
      setShowForm(false);
      clubs.refetch();
    } catch (e: any) {
      Alert.alert("Ops", e.message || "Erro ao criar clube.");
    } finally {
      setBusy(false);
    }
  };

  return (
    <View style={s.wrap}>
      <View style={s.head}>
        <View style={{ flexDirection: "row", justifyContent: "space-between", alignItems: "center" }}>
          <Text style={s.title}>Clubes de Corrida</Text>
          <TouchableOpacity style={s.newBtn} onPress={() => setShowForm((v) => !v)}>
            <Text style={s.newBtnTxt}>{showForm ? "Cancelar" : "+ Criar"}</Text>
          </TouchableOpacity>
        </View>
        <Text style={s.subtitle}>Grupos e equipes em Fortaleza</Text>
      </View>

      <FlatList
        data={clubs.data?.clubs ?? []}
        keyExtractor={(c) => c.id}
        contentContainerStyle={s.list}
        ListHeaderComponent={
          showForm ? (
            <View style={s.form}>
              <Text style={s.formLabel}>Nome do clube</Text>
              <TextInput
                style={s.input}
                placeholder="Ex: Aldeota Runners"
                placeholderTextColor={C.ink3}
                value={name}
                onChangeText={setName}
              />
              <Text style={s.formLabel}>Descrição</Text>
              <TextInput
                style={[s.input, { height: 70, textAlignVertical: "top" }]}
                placeholder="Horários de treino, local de encontro…"
                placeholderTextColor={C.ink3}
                multiline
                value={desc}
                onChangeText={setDesc}
              />
              <Text style={s.formLabel}>Cor</Text>
              <View style={{ flexDirection: "row", gap: 8, marginBottom: 12 }}>
                {COLORS.map((c) => (
                  <TouchableOpacity
                    key={c}
                    onPress={() => setColor(c)}
                    style={[s.swatch, { backgroundColor: c }, color === c && s.swatchOn]}
                  />
                ))}
              </View>
              <TouchableOpacity style={[s.saveBtn, busy && { opacity: 0.5 }]} disabled={busy} onPress={create}>
                <Text style={s.saveTxt}>{busy ? "Criando…" : "Salvar clube"}</Text>
              </TouchableOpacity>
            </View>
          ) : null
        }
        ListEmptyComponent={
          clubs.isLoading ? <ActivityIndicator color={C.teal} style={{ marginTop: 16 }} /> : (
            <Text style={s.empty}>Nenhum clube ainda — seja o primeiro a criar.</Text>
          )
        }
        renderItem={({ item }) => (
          <View style={[s.card, { borderLeftColor: item.color_hex, borderLeftWidth: 5 }]}>
            <View style={s.cardHeader}>
              <Text style={s.clubName}>{item.name}</Text>
              <Text style={s.membersBadge}>
                {item.member_count} {item.member_count === 1 ? "membro" : "membros"}
              </Text>
            </View>
            {item.description ? <Text style={s.desc}>{item.description}</Text> : null}
            <View style={s.cardFooter}>
              <Text style={s.statusText}>{item.is_member ? "✓ Você é membro" : "Aberto a novos membros"}</Text>
              {!item.is_member && (
                <TouchableOpacity style={s.btnJoin} onPress={() => join(item.id)}>
                  <Text style={s.btnText}>Entrar</Text>
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
  newBtn: { backgroundColor: C.teal, borderRadius: 8, paddingHorizontal: 12, paddingVertical: 6 },
  newBtnTxt: { color: "#fff", fontWeight: "700", fontSize: 12 },
  list: { padding: 16, gap: 12, paddingBottom: 40 },
  form: { backgroundColor: C.surface, borderRadius: 12, borderWidth: 1, borderColor: C.line, padding: 14, marginBottom: 4 },
  formLabel: { fontSize: 12, fontWeight: "700", color: C.ink, marginBottom: 5 },
  input: {
    borderWidth: 1,
    borderColor: C.line,
    borderRadius: 8,
    paddingHorizontal: 10,
    paddingVertical: 8,
    color: C.ink,
    backgroundColor: C.bg,
    marginBottom: 12,
  },
  swatch: { width: 30, height: 30, borderRadius: 15, borderWidth: 2, borderColor: "transparent" },
  swatchOn: { borderColor: C.ink },
  saveBtn: { backgroundColor: C.teal, borderRadius: 8, paddingVertical: 10, alignItems: "center" },
  saveTxt: { color: "#fff", fontWeight: "700" },
  card: { backgroundColor: C.surface, borderRadius: 12, borderWidth: 1, borderColor: C.line, padding: 14 },
  cardHeader: { flexDirection: "row", justifyContent: "space-between", alignItems: "center", gap: 8 },
  clubName: { fontSize: 16, fontWeight: "700", color: C.ink, flex: 1 },
  membersBadge: { fontSize: 11, color: C.ink3, backgroundColor: C.line, paddingHorizontal: 8, paddingVertical: 2, borderRadius: 10 },
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
  statusText: { fontSize: 12, color: C.ink3 },
  btnJoin: { backgroundColor: C.teal, borderRadius: 6, paddingHorizontal: 14, paddingVertical: 6 },
  btnText: { color: "#fff", fontWeight: "700", fontSize: 12 },
  empty: { padding: 24, color: C.ink3, fontSize: 13, textAlign: "center" },
});
