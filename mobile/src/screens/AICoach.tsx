import React, { useState } from "react";
import { View, Text, StyleSheet, TextInput, TouchableOpacity, ScrollView, ActivityIndicator } from "react-native";
import { useQuery } from "@tanstack/react-query";
import { api } from "../lib/api";
import { C } from "../theme";

type CoachResp = {
  message: string;
  tips: string[];
  source: "ia" | "fallback";
  disclaimer?: string;
};

export const AICoachScreen: React.FC = () => {
  const [prompt, setPrompt] = useState("");
  const [loading, setLoading] = useState(false);
  const [data, setData] = useState<CoachResp | null>(null);
  const summary = useQuery({ queryKey: ["coachSummary"], queryFn: api.coachSummary });

  async function handleAsk() {
    if (!prompt.trim()) return;
    setLoading(true);
    try {
      setData((await api.askAICoach(prompt.trim())).coach);
    } catch {
      /* ignora */
    } finally {
      setLoading(false);
    }
  }

  return (
    <ScrollView style={s.container} contentContainerStyle={{ paddingBottom: 40 }}>
      <Text style={s.title}>Coach IA</Text>
      <Text style={s.sub}>
        Sugestões a partir dos seus dados — não são prescrição de treino.
      </Text>

      {summary.data && (
        <View style={s.summary}>
          <Text style={s.summaryTitle}>Seus últimos 30 dias</Text>
          <Text style={s.summaryLine}>
            {summary.data.runs} corridas · {summary.data.distance_km} km ·{" "}
            {summary.data.moving_hours} h · +{summary.data.elevation_m} m
          </Text>
        </View>
      )}

      <View style={s.inputBox}>
        <TextInput
          style={s.input}
          placeholder="Ex.: melhor horário pra correr na Beira-Mar hoje?"
          placeholderTextColor={C.ink3}
          value={prompt}
          maxLength={1000}
          onChangeText={setPrompt}
        />
        <TouchableOpacity style={s.btn} onPress={handleAsk} disabled={loading}>
          {loading ? <ActivityIndicator color="#fff" /> : <Text style={s.btnText}>Perguntar</Text>}
        </TouchableOpacity>
      </View>

      {data && (
        <View style={s.card}>
          <Text style={s.message}>{data.message}</Text>
          {data.tips?.length > 0 && (
            <View style={s.tipsBox}>
              {data.tips.map((t, i) => (
                <Text key={i} style={s.tipItem}>• {t}</Text>
              ))}
            </View>
          )}
          {data.disclaimer ? <Text style={s.disclaimer}>{data.disclaimer}</Text> : null}
          <Text style={s.src}>{data.source === "ia" ? "gerado por IA" : "resposta padrão"}</Text>
        </View>
      )}
    </ScrollView>
  );
};

const s = StyleSheet.create({
  container: { flex: 1, backgroundColor: C.bg, padding: 16 },
  title: { fontSize: 22, fontWeight: "800", color: C.ink, marginTop: 12 },
  sub: { fontSize: 13, color: C.ink3, marginBottom: 16 },
  summary: { backgroundColor: C.surface, borderWidth: 1, borderColor: C.line, borderRadius: 12, padding: 12, marginBottom: 16 },
  summaryTitle: { fontSize: 11, color: C.ink3, textTransform: "uppercase", letterSpacing: 1 },
  summaryLine: { fontSize: 14, color: C.ink, fontWeight: "600", marginTop: 4 },
  inputBox: { gap: 10, marginBottom: 20 },
  input: { backgroundColor: C.surface, borderWidth: 1, borderColor: C.line, padding: 12, borderRadius: 10, fontSize: 14, color: C.ink },
  btn: { backgroundColor: C.teal, padding: 14, borderRadius: 10, alignItems: "center" },
  btnText: { color: "#fff", fontWeight: "700", fontSize: 14 },
  card: { backgroundColor: C.surface, borderRadius: 12, padding: 16, borderWidth: 1, borderColor: C.line, gap: 10 },
  message: { color: C.ink, fontSize: 14, lineHeight: 21 },
  tipsBox: { gap: 4 },
  tipItem: { color: C.ink2, fontSize: 13 },
  disclaimer: { color: C.coral, fontSize: 12, borderLeftWidth: 3, borderLeftColor: C.coral, paddingLeft: 8 },
  src: { color: C.ink3, fontSize: 11 },
});
