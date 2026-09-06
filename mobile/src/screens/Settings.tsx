import { useState } from "react";
import { Alert, ScrollView, StyleSheet, Text, TextInput, TouchableOpacity, View } from "react-native";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "../lib/api";
import { C } from "../theme";

const KINDS = ["watch", "footpod", "hrm"] as const;

export function SettingsScreen() {
  const qc = useQueryClient();
  const devices = useQuery({ queryKey: ["devices"], queryFn: api.devices });
  const badges = useQuery({ queryKey: ["badges"], queryFn: api.badges });
  const sub = useQuery({ queryKey: ["subscription"], queryFn: api.subscription });

  const [kind, setKind] = useState<(typeof KINDS)[number]>("watch");
  const [brand, setBrand] = useState("");
  const [model, setModel] = useState("");

  const reload = (k: string) => qc.invalidateQueries({ queryKey: [k] });

  async function addDevice() {
    if (!brand.trim()) return;
    try {
      await api.addDevice(kind, brand.trim(), model.trim());
      setBrand(""); setModel("");
      reload("devices");
    } catch (e: any) {
      Alert.alert("Ops", e?.message ?? "Erro ao parear");
    }
  }

  async function togglePremium() {
    try {
      if (sub.data?.active) {
        await api.cancelSubscription();
      } else {
        await api.subscribe("premium_monthly");
      }
      reload("subscription");
    } catch (e: any) {
      Alert.alert("Ops", e?.message ?? "Erro");
    }
  }

  return (
    <ScrollView style={s.wrap} contentContainerStyle={{ padding: 16, gap: 20, paddingBottom: 40 }}>
      <Text style={s.h1}>Ajustes</Text>

      <View style={s.card}>
        <Text style={s.section}>Assinatura premium</Text>
        <Text style={s.meta}>
          {sub.data?.active
            ? `Ativa · ${sub.data.plan ?? ""}`
            : "Histórico ilimitado, análises avançadas, cosméticos — nunca vantagem de conquista."}
        </Text>
        <TouchableOpacity style={sub.data?.active ? s.btnGhost : s.btn} onPress={togglePremium}>
          <Text style={sub.data?.active ? s.btnGhostText : s.btnText}>
            {sub.data?.active ? "Cancelar" : "Assinar (sandbox)"}
          </Text>
        </TouchableOpacity>
      </View>

      <View style={s.card}>
        <Text style={s.section}>Dispositivos pareados</Text>
        {(devices.data?.devices ?? []).map((d) => (
          <View key={d.id} style={s.row}>
            <Text style={s.rowText}>{d.kind} · {d.brand} {d.model}</Text>
            <TouchableOpacity onPress={() => api.deleteDevice(d.id).then(() => reload("devices"))}>
              <Text style={s.remove}>remover</Text>
            </TouchableOpacity>
          </View>
        ))}
        <View style={s.kindRow}>
          {KINDS.map((k) => (
            <TouchableOpacity key={k} style={[s.chip, kind === k && s.chipOn]} onPress={() => setKind(k)}>
              <Text style={[s.chipText, kind === k && s.chipTextOn]}>{k}</Text>
            </TouchableOpacity>
          ))}
        </View>
        <TextInput style={s.input} placeholder="Marca (Garmin, Coros…)" placeholderTextColor={C.ink3} value={brand} onChangeText={setBrand} />
        <TextInput style={s.input} placeholder="Modelo" placeholderTextColor={C.ink3} value={model} onChangeText={setModel} />
        <TouchableOpacity style={s.btn} onPress={addDevice}>
          <Text style={s.btnText}>Parear</Text>
        </TouchableOpacity>
      </View>

      <View style={s.card}>
        <Text style={s.section}>Sala de troféus</Text>
        {(badges.data?.badges ?? []).length === 0 && <Text style={s.meta}>Nenhum selo ainda.</Text>}
        {(badges.data?.badges ?? []).map((b) => (
          <View key={b.code} style={s.row}>
            <Text style={s.rowText}>🏅 {b.name}</Text>
            <Text style={s.meta}>{new Date(b.earned_at).toLocaleDateString("pt-BR")}</Text>
          </View>
        ))}
      </View>
    </ScrollView>
  );
}

const s = StyleSheet.create({
  wrap: { flex: 1, backgroundColor: C.bg },
  h1: { fontSize: 22, fontWeight: "800", color: C.ink },
  card: { backgroundColor: C.surface, borderWidth: 1, borderColor: C.line, borderRadius: 12, padding: 14, gap: 10 },
  section: { fontWeight: "700", color: C.ink, fontSize: 15 },
  meta: { color: C.ink3, fontSize: 12 },
  row: { flexDirection: "row", justifyContent: "space-between", alignItems: "center" },
  rowText: { color: C.ink, fontSize: 13 },
  remove: { color: C.crit, fontSize: 12, fontWeight: "600" },
  kindRow: { flexDirection: "row", gap: 8 },
  chip: { borderWidth: 1, borderColor: C.line, borderRadius: 999, paddingHorizontal: 12, paddingVertical: 5 },
  chipOn: { backgroundColor: C.teal, borderColor: C.teal },
  chipText: { color: C.ink2, fontSize: 12 },
  chipTextOn: { color: "#fff", fontWeight: "700" },
  input: { backgroundColor: C.bg, borderWidth: 1, borderColor: C.line, borderRadius: 10, padding: 10, color: C.ink },
  btn: { backgroundColor: C.teal, borderRadius: 10, paddingVertical: 11, alignItems: "center" },
  btnText: { color: "#fff", fontWeight: "700" },
  btnGhost: { borderWidth: 1.5, borderColor: C.line, borderRadius: 10, paddingVertical: 11, alignItems: "center" },
  btnGhostText: { color: C.ink, fontWeight: "700" },
});
