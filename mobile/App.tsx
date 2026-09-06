import { useState } from "react";
import { ActivityIndicator, Pressable, StyleSheet, Text, TextInput, View } from "react-native";
import { StatusBar } from "expo-status-bar";
import { QueryClient, QueryClientProvider, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, isAuthed, ApiError, type PublicUser } from "@/lib/api";

// App do corredor — Fase 0: login/cadastro + "quem sou eu".
// Mapa (MapLibre), gravação de corrida em background e território entram na Fase 1.

const qc = new QueryClient();

export default function App() {
  return (
    <QueryClientProvider client={qc}>
      <StatusBar style="auto" />
      <Root />
    </QueryClientProvider>
  );
}

function Root() {
  const client = useQueryClient();
  const authed = useQuery({ queryKey: ["authed"], queryFn: isAuthed });
  const me = useQuery<PublicUser>({
    queryKey: ["me"],
    queryFn: api.me,
    enabled: authed.data === true,
    retry: false,
  });

  if (authed.isLoading) return <Center><ActivityIndicator /></Center>;

  if (authed.data && me.data) {
    return (
      <Center>
        <Text style={styles.h1}>Olá, {me.data.username}</Text>
        <Text style={styles.muted}>ID {me.data.athlete_id}</Text>
        <Text style={styles.muted}>{me.data.email}</Text>
        <Pressable
          style={[styles.btn, styles.ghost]}
          onPress={async () => {
            await api.logout();
            client.clear();
          }}
        >
          <Text style={styles.ghostText}>Sair</Text>
        </Pressable>
        <Text style={[styles.muted, styles.small]}>
          Fase 0 — próximo: mapa, gravação de corrida em background, território.
        </Text>
      </Center>
    );
  }

  return <Auth onDone={() => client.invalidateQueries()} />;
}

function Auth({ onDone }: { onDone: () => void }) {
  const [mode, setMode] = useState<"login" | "register">("login");
  const [email, setEmail] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");

  const m = useMutation({
    mutationFn: () =>
      mode === "login" ? api.login(email, password) : api.register({ email, username, password }),
    onSuccess: onDone,
  });

  return (
    <Center>
      <Text style={styles.h1}>FortalRunners</Text>
      <View style={styles.seg}>
        {(["login", "register"] as const).map((mo) => (
          <Pressable key={mo} style={[styles.segBtn, mode === mo && styles.segOn]} onPress={() => setMode(mo)}>
            <Text style={[styles.segText, mode === mo && styles.segTextOn]}>
              {mo === "login" ? "Entrar" : "Criar conta"}
            </Text>
          </Pressable>
        ))}
      </View>

      <TextInput
        style={styles.input}
        placeholder="E-mail"
        autoCapitalize="none"
        keyboardType="email-address"
        value={email}
        onChangeText={setEmail}
      />
      {mode === "register" && (
        <TextInput
          style={styles.input}
          placeholder="Nome de usuário"
          autoCapitalize="none"
          value={username}
          onChangeText={setUsername}
        />
      )}
      <TextInput
        style={styles.input}
        placeholder="Senha"
        secureTextEntry
        value={password}
        onChangeText={setPassword}
      />

      {m.isError && (
        <Text style={styles.err}>
          {m.error instanceof ApiError ? m.error.message : "Falha na autenticação"}
        </Text>
      )}

      <Pressable style={[styles.btn, styles.primary]} disabled={m.isPending} onPress={() => m.mutate()}>
        <Text style={styles.primaryText}>
          {m.isPending ? "…" : mode === "login" ? "Entrar" : "Criar conta"}
        </Text>
      </Pressable>
    </Center>
  );
}

function Center({ children }: { children: React.ReactNode }) {
  return <View style={styles.center}>{children}</View>;
}

const styles = StyleSheet.create({
  center: { flex: 1, alignItems: "center", justifyContent: "center", gap: 12, padding: 24, backgroundColor: "#F4F1E8" },
  h1: { fontSize: 24, fontWeight: "800", color: "#16262A" },
  muted: { color: "#7B8688" },
  small: { fontSize: 12, textAlign: "center", marginTop: 12 },
  seg: { flexDirection: "row", backgroundColor: "#ECE6D9", borderRadius: 999, padding: 3, marginVertical: 8 },
  segBtn: { paddingVertical: 8, paddingHorizontal: 18, borderRadius: 999 },
  segOn: { backgroundColor: "#fff" },
  segText: { color: "#7B8688", fontWeight: "600" },
  segTextOn: { color: "#0C7F86" },
  input: {
    width: 280,
    borderWidth: 1.5,
    borderColor: "#CFC7B4",
    borderRadius: 8,
    paddingHorizontal: 12,
    paddingVertical: 10,
    backgroundColor: "#fff",
    color: "#16262A",
  },
  btn: { width: 280, paddingVertical: 13, borderRadius: 10, alignItems: "center", marginTop: 4 },
  primary: { backgroundColor: "#E0562F" },
  primaryText: { color: "#fff", fontWeight: "700" },
  ghost: { borderWidth: 1.5, borderColor: "#CFC7B4" },
  ghostText: { color: "#16262A", fontWeight: "700" },
  err: { color: "#C7402B", fontSize: 13 },
});
