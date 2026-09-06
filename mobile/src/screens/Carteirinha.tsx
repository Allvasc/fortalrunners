import { useEffect, useRef, useState } from "react";
import { Alert, ScrollView, StyleSheet, Text, TouchableOpacity, View } from "react-native";
import * as Brightness from "expo-brightness";
import QRCode from "react-native-qrcode-svg";
import { useQuery } from "@tanstack/react-query";
import { api, type PublicUser } from "../lib/api";
import { C } from "../theme";

// Carteirinha: ID FortalRunners + QR rotativo (validade curta) + histórico de
// leituras. Sobe o brilho da tela ao máximo para leitura em posto (plano §3).
export function CarteirinhaScreen() {
  const me = useQuery<PublicUser>({ queryKey: ["me"], queryFn: api.me });
  const token = useQuery({
    queryKey: ["qrToken"],
    queryFn: () => api.qrToken(),
    refetchInterval: 35_000, // o token expira em ~45 s
  });
  const scans = useQuery({ queryKey: ["myScans"], queryFn: api.myScans });
  const [secsLeft, setSecsLeft] = useState(0);
  const prevBright = useRef<number | null>(null);

  useEffect(() => {
    (async () => {
      try {
        const { granted } = await Brightness.requestPermissionsAsync();
        if (granted) {
          prevBright.current = await Brightness.getBrightnessAsync();
          await Brightness.setBrightnessAsync(1);
        }
      } catch {}
    })();
    return () => {
      if (prevBright.current != null) Brightness.setBrightnessAsync(prevBright.current).catch(() => {});
    };
  }, []);

  useEffect(() => {
    const exp = token.data?.qr.expires_at ? new Date(token.data.qr.expires_at).getTime() : 0;
    const iv = setInterval(() => setSecsLeft(Math.max(0, Math.round((exp - Date.now()) / 1000))), 1000);
    return () => clearInterval(iv);
  }, [token.data]);

  const payload = token.data?.qr.payload_sig ?? "";

  const dispute = async (id: string) => {
    try {
      await api.disputeScan(id);
      Alert.alert("Contestação registrada", "Vamos revisar essa leitura.");
      scans.refetch();
    } catch (e: any) {
      Alert.alert("Ops", e.message || "Erro ao contestar.");
    }
  };

  return (
    <ScrollView style={{ backgroundColor: C.bg }} contentContainerStyle={s.wrap}>
      <View style={s.card}>
        <Text style={s.name}>{me.data?.username ?? "corredor"}</Text>
        <Text style={s.aid}>{me.data?.athlete_id}</Text>

        <View style={s.qrBox}>
          {payload ? (
            <QRCode value={payload} size={220} backgroundColor="#fff" color="#16262A" />
          ) : (
            <Text style={s.muted}>gerando…</Text>
          )}
        </View>

        <Text style={s.rotate}>
          {secsLeft > 0 ? `renova em ${secsLeft}s` : "renovando…"}
        </Text>
        <Text style={s.hint}>
          Mostre este código no posto do evento. Ele muda sozinho a cada leitura — não tire print.
        </Text>
      </View>

      <Text style={s.section}>Últimas leituras</Text>
      {(scans.data?.scans ?? []).length === 0 && (
        <Text style={s.muted}>Nenhuma leitura ainda.</Text>
      )}
      {(scans.data?.scans ?? []).map((sc) => (
        <View key={sc.id} style={s.scanRow}>
          <View style={{ flex: 1 }}>
            <Text style={s.scanKind}>{sc.kind}</Text>
            <Text style={s.muted}>
              {new Date(sc.scanned_at).toLocaleString("pt-BR")} · {sc.status}
            </Text>
          </View>
          <TouchableOpacity onPress={() => dispute(sc.id)}>
            <Text style={s.disputeLink}>contestar</Text>
          </TouchableOpacity>
        </View>
      ))}
    </ScrollView>
  );
}

const s = StyleSheet.create({
  wrap: { padding: 20, gap: 14, paddingBottom: 40, alignItems: "stretch" },
  card: {
    backgroundColor: C.surface,
    borderRadius: 18,
    borderWidth: 1,
    borderColor: C.line,
    padding: 20,
    alignItems: "center",
    gap: 8,
  },
  name: { fontSize: 20, fontWeight: "800", color: C.ink },
  aid: { fontSize: 14, fontWeight: "700", color: C.teal, letterSpacing: 1 },
  qrBox: {
    backgroundColor: "#fff",
    padding: 14,
    borderRadius: 12,
    marginTop: 6,
    minHeight: 248,
    minWidth: 248,
    alignItems: "center",
    justifyContent: "center",
  },
  rotate: { fontSize: 12, fontWeight: "700", color: C.ink3, fontVariant: ["tabular-nums"] },
  hint: { fontSize: 12, color: C.ink3, textAlign: "center", lineHeight: 16, maxWidth: 260 },
  section: { fontWeight: "800", color: C.ink, fontSize: 16, marginTop: 8 },
  muted: { color: C.ink3, fontSize: 13 },
  scanRow: {
    flexDirection: "row",
    alignItems: "center",
    backgroundColor: C.surface,
    borderRadius: 10,
    borderWidth: 1,
    borderColor: C.line,
    padding: 12,
  },
  scanKind: { fontSize: 13, fontWeight: "700", color: C.ink },
  disputeLink: { color: C.coral, fontSize: 12, fontWeight: "700" },
});
