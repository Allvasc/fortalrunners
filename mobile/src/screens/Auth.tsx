import { useState } from "react";
import { ActivityIndicator, Image, Pressable, StyleSheet, Text, TextInput, View } from "react-native";
import { Ionicons } from "@expo/vector-icons";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api, ApiError, oauthLogin } from "../lib/api";
import { C } from "../theme";

export function Auth() {
  const qc = useQueryClient();
  const [mode, setMode] = useState<"login" | "register">("login");
  const [email, setEmail] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [mfaToken, setMfaToken] = useState<string | null>(null);
  const [code, setCode] = useState("");

  const m = useMutation({
    mutationFn: async () => {
      if (mode === "register") {
        await api.register({ email, username, password });
        return;
      }
      const r = await api.login(email, password);
      if (r.mfa) setMfaToken(r.mfaToken);
    },
    onSuccess: () => {
      if (!mfaToken) qc.invalidateQueries();
    },
  });

  const verify = useMutation({
    mutationFn: () => api.verifyMfa(mfaToken!, code.trim()),
    onSuccess: () => qc.invalidateQueries(),
  });

  const google = useMutation({
    mutationFn: () => oauthLogin("google"),
    onSuccess: () => {
      qc.clear();
      qc.setQueryData(["authed"], true);
    },
  });

  if (mfaToken) {
    return (
      <View style={s.center}>
        <Text style={s.h1}>Verificação em 2 etapas</Text>
        <Text style={{ color: C.ink3, textAlign: "center", maxWidth: 280 }}>
          Código de 6 dígitos do seu app de autenticação — ou um código de recuperação.
        </Text>
        <TextInput
          style={s.input}
          placeholder="123456"
          autoCapitalize="none"
          value={code}
          onChangeText={setCode}
          autoFocus
        />
        {verify.isError && (
          <Text style={s.err}>{verify.error instanceof ApiError ? verify.error.message : "Código inválido"}</Text>
        )}
        <Pressable style={s.btn} disabled={verify.isPending} onPress={() => verify.mutate()}>
          <Text style={s.btnText}>{verify.isPending ? "Verificando…" : "Entrar"}</Text>
        </Pressable>
        <Pressable
          onPress={() => {
            setMfaToken(null);
            setCode("");
          }}
        >
          <Text style={{ color: C.ink3 }}>Voltar</Text>
        </Pressable>
      </View>
    );
  }

  return (
    <View style={s.center}>
      <Image source={require("../../assets/icon.png")} style={s.logo} />
      <Text style={s.h1}>FortalRunners</Text>

      <Pressable style={s.gbtn} disabled={google.isPending} onPress={() => google.mutate()}>
        {google.isPending ? (
          <ActivityIndicator color={C.ink} />
        ) : (
          <>
            <Ionicons name="logo-google" size={18} color="#EA4335" />
            <Text style={s.gbtnText}>Entrar com Google</Text>
          </>
        )}
      </Pressable>
      {google.isError && (
        <Text style={s.err}>
          {google.error instanceof ApiError && google.error.message !== "Login cancelado"
            ? google.error.message
            : "Não foi possível entrar com o Google."}
        </Text>
      )}

      <View style={s.orRow}>
        <View style={s.orLine} />
        <Text style={s.orText}>ou</Text>
        <View style={s.orLine} />
      </View>

      <View style={s.seg}>
        {(["login", "register"] as const).map((mo) => (
          <Pressable key={mo} style={[s.segBtn, mode === mo && s.segOn]} onPress={() => setMode(mo)}>
            <Text style={[s.segText, mode === mo && s.segTextOn]}>
              {mo === "login" ? "Entrar" : "Criar conta"}
            </Text>
          </Pressable>
        ))}
      </View>

      <TextInput style={s.input} placeholder="E-mail" autoCapitalize="none" keyboardType="email-address" value={email} onChangeText={setEmail} />
      {mode === "register" && (
        <TextInput style={s.input} placeholder="Nome de usuário" autoCapitalize="none" value={username} onChangeText={setUsername} />
      )}
      <TextInput style={s.input} placeholder="Senha" secureTextEntry value={password} onChangeText={setPassword} />

      {m.isError && (
        <Text style={s.err}>{m.error instanceof ApiError ? m.error.message : "Falha na autenticação"}</Text>
      )}

      <Pressable style={s.btn} disabled={m.isPending} onPress={() => m.mutate()}>
        <Text style={s.btnText}>{m.isPending ? "…" : mode === "login" ? "Entrar" : "Criar conta"}</Text>
      </Pressable>
    </View>
  );
}

const s = StyleSheet.create({
  center: { flex: 1, alignItems: "center", justifyContent: "center", gap: 12, padding: 24, backgroundColor: C.bg },
  logo: { width: 64, height: 64, borderRadius: 16, marginBottom: 4 },
  h1: { fontSize: 26, fontWeight: "800", color: C.ink, marginBottom: 4 },
  gbtn: {
    width: 280,
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "center",
    gap: 10,
    paddingVertical: 12,
    borderRadius: 10,
    borderWidth: 1.5,
    borderColor: C.line,
    backgroundColor: C.surface,
  },
  gbtnText: { color: C.ink, fontWeight: "700" },
  orRow: { flexDirection: "row", alignItems: "center", gap: 10, width: 280, marginVertical: 2 },
  orLine: { flex: 1, height: 1, backgroundColor: C.line },
  orText: { color: C.ink3, fontSize: 12 },
  seg: { flexDirection: "row", backgroundColor: C.sunken, borderRadius: 999, padding: 3, marginVertical: 8 },
  segBtn: { paddingVertical: 8, paddingHorizontal: 18, borderRadius: 999 },
  segOn: { backgroundColor: C.surface },
  segText: { color: C.ink3, fontWeight: "600" },
  segTextOn: { color: C.tealDeep },
  input: { width: 280, borderWidth: 1.5, borderColor: C.line, borderRadius: 8, paddingHorizontal: 12, paddingVertical: 10, backgroundColor: C.surface, color: C.ink },
  btn: { width: 280, paddingVertical: 13, borderRadius: 10, alignItems: "center", marginTop: 4, backgroundColor: C.coral },
  btnText: { color: "#fff", fontWeight: "700" },
  err: { color: C.crit, fontSize: 13 },
});
