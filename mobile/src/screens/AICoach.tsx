import React, { useState } from "react";
import { View, Text, StyleSheet, TextInput, TouchableOpacity, ScrollView, ActivityIndicator } from "react-native";
import { api } from "../lib/api";

type CoachResp = {
  message: string;
  tips: string[];
  weather: string;
};

export const AICoachScreen: React.FC = () => {
  const [prompt, setPrompt] = useState("");
  const [loading, setLoading] = useState(false);
  const [data, setData] = useState<CoachResp | null>(null);

  async function handleAsk() {
    if (!prompt.trim()) return;
    setLoading(true);
    try {
      const res = await api.askAICoach(prompt);
      setData(res.coach);
    } catch {
      /* ignora */
    } finally {
      setLoading(false);
    }
  }

  return (
    <ScrollView style={styles.container}>
      <Text style={styles.title}>🤖 Coach IA FortalRunners</Text>
      <Text style={styles.sub}>Dicas personalizadas de clima, hidratação e ritmo para treinos em Fortaleza.</Text>

      <View style={styles.inputBox}>
        <TextInput
          style={styles.input}
          placeholder="Ex: Como treinar na Beira-Mar com vento forte?"
          value={prompt}
          onChangeText={setPrompt}
        />
        <TouchableOpacity style={styles.btn} onPress={handleAsk} disabled={loading}>
          {loading ? (
            <ActivityIndicator color="#FFF" />
          ) : (
            <Text style={styles.btnText}>Perguntar</Text>
          )}
        </TouchableOpacity>
      </View>

      {data && (
        <View style={styles.card}>
          <Text style={styles.cardHeader}>🏃‍♂️ Orientação do Coach</Text>
          <Text style={styles.message}>{data.message}</Text>

          {data.tips && data.tips.length > 0 && (
            <View style={styles.tipsBox}>
              <Text style={styles.tipsTitle}>💡 DICAS PRÁTICAS</Text>
              {data.tips.map((t, idx) => (
                <Text key={idx} style={styles.tipItem}>
                  • {t}
                </Text>
              ))}
            </View>
          )}
        </View>
      )}
    </ScrollView>
  );
};

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: "#F4F1E8", padding: 16 },
  title: { fontSize: 22, fontWeight: "bold", color: "#0C7F86", marginTop: 24 },
  sub: { fontSize: 13, color: "#666", marginBottom: 16 },
  inputBox: { gap: 10, marginBottom: 20 },
  input: { backgroundColor: "#FFF", borderWidth: 1, borderColor: "#CCC", padding: 12, borderRadius: 8, fontSize: 14 },
  btn: { backgroundColor: "#0C7F86", padding: 14, borderRadius: 8, alignItems: "center" },
  btnText: { color: "#FFF", fontWeight: "bold", fontSize: 14 },
  card: { backgroundColor: "#06181A", borderRadius: 12, padding: 16, borderLeftWidth: 4, borderLeftColor: "#3CB4BE" },
  cardHeader: { color: "#3CB4BE", fontWeight: "bold", fontSize: 16, marginBottom: 8 },
  message: { color: "#FFF", fontSize: 14, lineHeight: 20, marginBottom: 12 },
  tipsBox: { backgroundColor: "rgba(255,255,255,0.05)", padding: 10, borderRadius: 8 },
  tipsTitle: { color: "#E3B657", fontWeight: "bold", fontSize: 11, marginBottom: 6 },
  tipItem: { color: "#E9E4D7", fontSize: 12, marginBottom: 4 },
});
