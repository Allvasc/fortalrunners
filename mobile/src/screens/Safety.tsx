import { useState } from "react";
import {
  Alert, FlatList, StyleSheet, Text, TextInput, TouchableOpacity, View,
} from "react-native";
import * as Location from "expo-location";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "../lib/api";
import { C } from "../theme";

export function SafetyScreen() {
  const qc = useQueryClient();
  const contacts = useQuery({ queryKey: ["safetyContacts"], queryFn: api.safetyContacts });
  const [name, setName] = useState("");
  const [phone, setPhone] = useState("");
  const [relation, setRelation] = useState("");
  const [activeSOS, setActiveSOS] = useState<string | null>(null);

  const reload = () => qc.invalidateQueries({ queryKey: ["safetyContacts"] });

  async function addContact() {
    if (!name.trim() || !phone.trim()) return;
    try {
      await api.addSafetyContact(name.trim(), phone.trim(), relation.trim());
      setName(""); setPhone(""); setRelation("");
      reload();
    } catch (e: any) {
      Alert.alert("Ops", e?.message ?? "Erro ao adicionar contato");
    }
  }

  async function fireSOS() {
    const perm = await Location.getForegroundPermissionsAsync();
    if (!perm.granted) {
      const req = await Location.requestForegroundPermissionsAsync();
      if (!req.granted) {
        Alert.alert("Sem localização", "O SOS precisa da sua posição.");
        return;
      }
    }
    const pos = await Location.getCurrentPositionAsync({ accuracy: Location.Accuracy.High });
    try {
      const r = await api.triggerSOS(pos.coords.latitude, pos.coords.longitude, "SOS pelo app");
      setActiveSOS(r.id);
      Alert.alert(
        "SOS enviado",
        `${r.notified_contacts} contato(s) avisado(s).\nLink de acompanhamento: ${r.share_url ?? r.share_token}\n\nIsto NÃO aciona a polícia (190) nem o SAMU (192).`,
      );
    } catch (e: any) {
      Alert.alert("Falhou", e?.message ?? "Não foi possível enviar o SOS");
    }
  }

  async function cancelSOS() {
    if (!activeSOS) return;
    try {
      await api.cancelSOS(activeSOS);
      setActiveSOS(null);
      Alert.alert("SOS encerrado", "O alerta foi cancelado.");
    } catch (e: any) {
      Alert.alert("Ops", e?.message ?? "Erro ao cancelar");
    }
  }

  return (
    <View style={s.wrap}>
      <FlatList
        data={contacts.data?.contacts ?? []}
        keyExtractor={(c) => c.id}
        contentContainerStyle={s.list}
        ListHeaderComponent={
          <>
            <Text style={s.h1}>Segurança</Text>
            <Text style={s.sub}>
              O SOS envia a sua localização aos contatos de emergência. Não substitui 190 / 192.
            </Text>

            <TouchableOpacity
              style={[s.sos, activeSOS ? s.sosActive : null]}
              onPress={activeSOS ? cancelSOS : fireSOS}
            >
              <Text style={s.sosText}>{activeSOS ? "Encerrar SOS" : "Enviar SOS agora"}</Text>
            </TouchableOpacity>

            <Text style={s.section}>Contatos de emergência</Text>
            <View style={s.form}>
              <TextInput style={s.input} placeholder="Nome" placeholderTextColor={C.ink3} value={name} onChangeText={setName} />
              <TextInput style={s.input} placeholder="Telefone" placeholderTextColor={C.ink3} keyboardType="phone-pad" value={phone} onChangeText={setPhone} />
              <TextInput style={s.input} placeholder="Relação (mãe, amigo…)" placeholderTextColor={C.ink3} value={relation} onChangeText={setRelation} />
              <TouchableOpacity style={s.addBtn} onPress={addContact}>
                <Text style={s.addText}>Adicionar</Text>
              </TouchableOpacity>
            </View>
          </>
        }
        renderItem={({ item }) => (
          <View style={s.contact}>
            <View style={{ flex: 1 }}>
              <Text style={s.cName}>{item.name}</Text>
              <Text style={s.cMeta}>{item.phone}{item.relation ? ` · ${item.relation}` : ""}</Text>
            </View>
            <TouchableOpacity
              onPress={() => api.deleteSafetyContact(item.id).then(reload)}
            >
              <Text style={s.remove}>remover</Text>
            </TouchableOpacity>
          </View>
        )}
        ListEmptyComponent={<Text style={s.sub}>Nenhum contato ainda.</Text>}
      />
    </View>
  );
}

const s = StyleSheet.create({
  wrap: { flex: 1, backgroundColor: C.bg },
  list: { padding: 16, gap: 10 },
  h1: { fontSize: 22, fontWeight: "800", color: C.ink, marginTop: 8 },
  sub: { fontSize: 13, color: C.ink3, marginTop: 4 },
  sos: { backgroundColor: C.coral, borderRadius: 14, paddingVertical: 18, alignItems: "center", marginTop: 16 },
  sosActive: { backgroundColor: C.ink2 },
  sosText: { color: "#fff", fontWeight: "800", fontSize: 16 },
  section: { fontWeight: "700", color: C.ink, fontSize: 15, marginTop: 22, marginBottom: 8 },
  form: { gap: 8, marginBottom: 8 },
  input: { backgroundColor: C.surface, borderWidth: 1, borderColor: C.line, borderRadius: 10, padding: 11, color: C.ink },
  addBtn: { backgroundColor: C.teal, borderRadius: 10, paddingVertical: 11, alignItems: "center" },
  addText: { color: "#fff", fontWeight: "700" },
  contact: { flexDirection: "row", alignItems: "center", backgroundColor: C.surface, borderWidth: 1, borderColor: C.line, borderRadius: 10, padding: 12 },
  cName: { fontWeight: "700", color: C.ink },
  cMeta: { color: C.ink3, fontSize: 12, marginTop: 2 },
  remove: { color: C.crit, fontSize: 12, fontWeight: "600" },
});
