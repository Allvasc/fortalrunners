import { useEffect, useRef, useState } from "react";
import { Alert, StyleSheet, Text, TouchableOpacity, View } from "react-native";
import { useKeepAwake } from "expo-keep-awake";
import { useMutation } from "@tanstack/react-query";
import type { NativeStackScreenProps } from "@react-navigation/native-stack";
import { api, ApiError } from "../lib/api";
import {
  computeStats,
  readLive,
  setPaused,
  startRecording,
  stopRecording,
  type GPSPoint,
  type LiveStats,
} from "../lib/recorder";
import { TrackPreview } from "../components/TrackPreview";
import { C } from "../theme";
import { clock, km, pace } from "../format";
import type { RootStack } from "../../App";

type Props = NativeStackScreenProps<RootStack, "Recording">;

const EMPTY: LiveStats = { distanceM: 0, movingS: 0, elapsedS: 0, paceS: 0, points: [] };

export function Recording({ navigation }: Props) {
  useKeepAwake();
  const [phase, setPhase] = useState<"idle" | "recording" | "paused">("idle");
  const [stats, setStats] = useState<LiveStats>(EMPTY);
  const startedAt = useRef<number>(0);

  useEffect(() => {
    let alive = true;
    (async () => {
      const ok = await startRecording();
      if (!alive) return;
      if (!ok) {
        Alert.alert("Sem permissão de GPS", "Autorize a localização para gravar a corrida.", [
          { text: "Voltar", onPress: () => navigation.goBack() },
        ]);
        return;
      }
      startedAt.current = Date.now();
      setPhase("recording");
    })();
    return () => {
      alive = false;
    };
  }, [navigation]);

  useEffect(() => {
    if (phase === "idle") return;
    const tick = async () => {
      const { meta, points } = await readLive();
      setStats(computeStats(points, meta?.startedAt ?? startedAt.current));
    };
    tick();
    const iv = setInterval(tick, 2000);
    return () => clearInterval(iv);
  }, [phase]);

  const upload = useMutation({
    mutationFn: async (data: { startedAt: number; points: GPSPoint[]; cadence: { t: number; v: number }[] }) => {
      const start = new Date(data.startedAt).toISOString();
      const end = new Date(data.points[data.points.length - 1]?.t ?? Date.now()).toISOString();
      return api.uploadRun({ started_at: start, ended_at: end, points: data.points, cadence: data.cadence });
    },
    onSuccess: (run) => navigation.replace("Summary", { runId: run.id }),
    onError: (e) =>
      Alert.alert("Falha ao enviar", e instanceof ApiError ? e.message : "Tente de novo em Últimas corridas."),
  });

  const togglePause = async () => {
    const next = phase === "recording";
    await setPaused(next);
    setPhase(next ? "paused" : "recording");
  };

  const finish = async () => {
    const { startedAt: sa, points, cadence } = await stopRecording();
    setPhase("idle");
    if (computeStats(points, sa).distanceM < 50) {
      Alert.alert("Corrida muito curta", "Menos de 50 m — nada foi salvo.", [
        { text: "Ok", onPress: () => navigation.goBack() },
      ]);
      return;
    }
    upload.mutate({ startedAt: sa, points, cadence });
  };

  return (
    <View style={s.wrap}>
      <View style={s.big}>
        <Text style={s.bigV}>{km(stats.distanceM)}</Text>
        <Text style={s.bigL}>km</Text>
      </View>

      <View style={s.row}>
        <Metric v={clock(stats.movingS)} l="tempo" />
        <Metric v={pace(stats.paceS)} l="pace /km" />
        <Metric v={String(stats.points.length)} l="pontos GPS" />
      </View>

      <TrackPreview points={stats.points} size={240} />

      {phase === "paused" && <Text style={s.pausedTag}>pausado</Text>}

      <View style={s.controls}>
        <TouchableOpacity style={[s.ctrl, s.ghost]} onPress={togglePause} disabled={upload.isPending}>
          <Text style={s.ghostText}>{phase === "recording" ? "Pausar" : "Retomar"}</Text>
        </TouchableOpacity>
        <TouchableOpacity style={[s.ctrl, s.stop]} onPress={finish} disabled={upload.isPending}>
          <Text style={s.stopText}>{upload.isPending ? "Enviando…" : "Concluir"}</Text>
        </TouchableOpacity>
      </View>
    </View>
  );
}

function Metric({ v, l }: { v: string; l: string }) {
  return (
    <View style={{ alignItems: "center" }}>
      <Text style={s.mV}>{v}</Text>
      <Text style={s.mL}>{l}</Text>
    </View>
  );
}

const s = StyleSheet.create({
  wrap: { flex: 1, backgroundColor: C.bg, alignItems: "center", justifyContent: "center", gap: 22, padding: 24 },
  big: { alignItems: "center" },
  bigV: { fontSize: 68, fontWeight: "800", color: C.ink, fontVariant: ["tabular-nums"] },
  bigL: { fontSize: 15, color: C.ink3, marginTop: -6 },
  row: { flexDirection: "row", gap: 28 },
  mV: { fontSize: 22, fontWeight: "700", color: C.ink, fontVariant: ["tabular-nums"] },
  mL: { fontSize: 11, textTransform: "uppercase", color: C.ink3 },
  pausedTag: { color: C.coral, fontWeight: "700", textTransform: "uppercase", letterSpacing: 1 },
  controls: { flexDirection: "row", gap: 12 },
  ctrl: { flex: 1, minWidth: 140, paddingVertical: 16, borderRadius: 14, alignItems: "center" },
  ghost: { borderWidth: 1.5, borderColor: C.line },
  ghostText: { fontWeight: "700", color: C.ink },
  stop: { backgroundColor: C.coral },
  stopText: { fontWeight: "800", color: "#fff" },
});
