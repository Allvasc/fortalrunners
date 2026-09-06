import React, { useEffect, useState } from "react";
import { View, Text, StyleSheet, FlatList, TouchableOpacity, Alert, ActivityIndicator } from "react-native";
import { api } from "../lib/api";

type EventItem = {
  id: string;
  title: string;
  description: string;
  type: string;
  starts_at: string;
  ends_at: string;
  location_name: string;
  status: string;
  is_registered: boolean;
};

export const EventsScreen: React.FC = () => {
  const [events, setEvents] = useState<EventItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeQR, setActiveQR] = useState<{ sig: string; expires: string } | null>(null);

  useEffect(() => {
    loadEvents();
  }, []);

  async function loadEvents() {
    try {
      const res = await api.events();
      setEvents(res.events || []);
    } catch {
      /* ignora */
    } finally {
      setLoading(false);
    }
  }

  async function handleRegister(id: string) {
    try {
      const res = await api.registerEvent(id);
      Alert.alert("Sucesso!", res.message);
      loadEvents();
      fetchQR(id);
    } catch (e: any) {
      Alert.alert("Erro", e.message || "Falha na inscrição.");
    }
  }

  async function fetchQR(eventId?: string) {
    try {
      const res = await api.qrToken(eventId);
      setActiveQR({ sig: res.qr.payload_sig, expires: res.qr.expires_at });
    } catch {
      Alert.alert("Erro", "Não foi possível gerar o QR Code.");
    }
  }

  if (loading) {
    return (
      <View style={styles.center}>
        <ActivityIndicator size="large" color="#0C7F86" />
      </View>
    );
  }

  return (
    <View style={styles.container}>
      <Text style={styles.headerTitle}>🏁 Eventos de Fortaleza</Text>
      <Text style={styles.headerSub}>Corridas, provas rústicas e desafios de orla com passaporte digital.</Text>

      {activeQR && (
        <View style={styles.qrCard}>
          <Text style={styles.qrTitle}>🎫 QR Code Passaporte Digital</Text>
          <Text style={styles.qrSig}>{activeQR.sig}</Text>
          <Text style={styles.qrExpiry}>Válido até: {new Date(activeQR.expires).toLocaleTimeString()}</Text>
        </View>
      )}

      <FlatList
        data={events}
        keyExtractor={(item) => item.id}
        renderItem={({ item }) => (
          <View style={styles.card}>
            <View style={styles.badgeRow}>
              <Text style={styles.typeTag}>{item.type.toUpperCase()}</Text>
              <Text style={styles.statusTag}>{item.status.toUpperCase()}</Text>
            </View>

            <Text style={styles.title}>{item.title}</Text>
            <Text style={styles.location}>📍 {item.location_name}</Text>
            <Text style={styles.desc}>{item.description}</Text>

            <View style={styles.footer}>
              {item.is_registered ? (
                <TouchableOpacity style={styles.qrBtn} onPress={() => fetchQR(item.id)}>
                  <Text style={styles.qrBtnText}>📱 Ver QR Pass</Text>
                </TouchableOpacity>
              ) : (
                <TouchableOpacity style={styles.btn} onPress={() => handleRegister(item.id)}>
                  <Text style={styles.btnText}>Garantir Inscrição</Text>
                </TouchableOpacity>
              )}
            </View>
          </View>
        )}
      />
    </View>
  );
};

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: "#F4F1E8", padding: 16 },
  center: { flex: 1, justifyContent: "center", alignItems: "center" },
  headerTitle: { fontSize: 22, fontWeight: "bold", color: "#0C7F86", marginTop: 24 },
  headerSub: { fontSize: 13, color: "#666", marginBottom: 16 },
  qrCard: { backgroundColor: "#06181A", padding: 16, borderRadius: 12, marginBottom: 16, alignItems: "center" },
  qrTitle: { color: "#3CB4BE", fontWeight: "bold", fontSize: 16 },
  qrSig: { color: "#FFF", fontFamily: "monospace", fontSize: 14, marginVertical: 6 },
  qrExpiry: { color: "#888", fontSize: 11 },
  card: { backgroundColor: "#FFF", borderRadius: 12, padding: 16, marginBottom: 12, borderWidth: 1, borderColor: "#E4DED0" },
  badgeRow: { flexDirection: "row", justifyContent: "space-between", marginBottom: 6 },
  typeTag: { fontSize: 10, fontWeight: "bold", color: "#E0562F", backgroundColor: "#FFF0EB", paddingHorizontal: 8, paddingVertical: 2, borderRadius: 4 },
  statusTag: { fontSize: 10, fontWeight: "bold", color: "#2E8F63", backgroundColor: "#EAF7F0", paddingHorizontal: 8, paddingVertical: 2, borderRadius: 4 },
  title: { fontSize: 16, fontWeight: "bold", color: "#16262A", marginBottom: 4 },
  location: { fontSize: 12, color: "#555", marginBottom: 6 },
  desc: { fontSize: 13, color: "#444", marginBottom: 12 },
  footer: { borderTopWidth: 1, borderTopColor: "#EEE", paddingTop: 10, alignItems: "flex-end" },
  btn: { backgroundColor: "#E0562F", paddingHorizontal: 16, paddingVertical: 10, borderRadius: 8 },
  btnText: { color: "#FFF", fontWeight: "bold", fontSize: 13 },
  qrBtn: { backgroundColor: "#0C7F86", paddingHorizontal: 16, paddingVertical: 10, borderRadius: 8 },
  qrBtnText: { color: "#FFF", fontWeight: "bold", fontSize: 13 },
});
