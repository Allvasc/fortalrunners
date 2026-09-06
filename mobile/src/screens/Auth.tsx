import { useState } from "react";
import {
  ActivityIndicator,
  Image,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  TextInput,
  View,
} from "react-native";
import { Ionicons } from "@expo/vector-icons";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api, ApiError, oauthLogin } from "../lib/api";
import { AuroraBg } from "../ui/AuroraBg";
import { C } from "../theme";

type Screen = "auth" | "forgot" | "reset";

export function Auth() {
  const qc = useQueryClient();
  const [screen, setScreen] = useState<Screen>("auth");
  const [mode, setMode] = useState<"login" | "register">("login");
  const [email, setEmail] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [mfaToken, setMfaToken] = useState<string | null>(null);
  const [code, setCode] = useState("");
  const [resetToken, setResetToken] = useState("");
  const [newPass, setNewPass] = useState("");

  const done = () => {
    qc.setQueryData(["authed"], true);
    qc.invalidateQueries();
  };

  const m = useMutation({
    mutationFn: async () => {
      if (mode === "register") {
        await api.register({ email: email.trim(), username: username.trim(), password });
        return;
      }
      const r = await api.login(email.trim(), password);
      if (r.mfa) setMfaToken(r.mfaToken);
    },
    onSuccess: () => {
      if (!mfaToken) done();
    },
  });
  const verify = useMutation({ mutationFn: () => api.verifyMfa(mfaToken!, code.trim()), onSuccess: done });
  const google = useMutation({ mutationFn: () => oauthLogin("google"), onSuccess: done });
  const forgot = useMutation({ mutationFn: () => api.forgotPassword(email.trim()) });
  const reset = useMutation({
    mutationFn: () => api.resetPassword(resetToken.trim(), newPass),
    onSuccess: () => {
      setScreen("auth");
      setMode("login");
      setPassword(newPass);
    },
  });

  const errText = (e: unknown, fallback: string) =>
    e instanceof ApiError && e.message && e.message !== "Login cancelado" ? e.message : fallback;

  // --- 2FA ---
  if (mfaToken) {
    return (
      <Shell>
        <Text style={s.h1}>Verificação em 2 etapas</Text>
        <Text style={s.muted}>Código de 6 dígitos do seu app de autenticação — ou um código de recuperação.</Text>
        <TextInput style={s.input} placeholder="123456" autoCapitalize="none" value={code} onChangeText={setCode} autoFocus />
        {verify.isError && <ErrBanner text={errText(verify.error, "Código inválido.")} />}
        <Btn label={verify.isPending ? "Verificando…" : "Entrar"} disabled={verify.isPending} onPress={() => verify.mutate()} />
        <Link label="Voltar" onPress={() => { setMfaToken(null); setCode(""); }} />
      </Shell>
    );
  }

  // --- esqueci a senha ---
  if (screen === "forgot") {
    return (
      <Shell>
        <Text style={s.h1}>Recuperar senha</Text>
        <Text style={s.muted}>Digite seu e-mail. Se houver conta, enviamos um link para redefinir.</Text>
        <TextInput
          style={s.input}
          placeholder="E-mail"
          autoCapitalize="none"
          keyboardType="email-address"
          value={email}
          onChangeText={setEmail}
        />
        {forgot.isSuccess && (
          <Text style={s.ok}>Se este e-mail estiver cadastrado, o link chegou. Cole o código dele abaixo.</Text>
        )}
        {forgot.isError && <ErrBanner text="Não foi possível enviar. Tente de novo." />}
        <Btn
          label={forgot.isPending ? "Enviando…" : "Enviar link"}
          disabled={forgot.isPending || !email.trim()}
          onPress={() => forgot.mutate()}
        />
        <Link label="Já tenho o código →" onPress={() => setScreen("reset")} />
        <Link label="Voltar ao login" onPress={() => setScreen("auth")} />
      </Shell>
    );
  }

  // --- redefinir com token ---
  if (screen === "reset") {
    return (
      <Shell>
        <Text style={s.h1}>Nova senha</Text>
        <Text style={s.muted}>Cole o código do link que você recebeu e escolha uma senha nova.</Text>
        <TextInput style={s.input} placeholder="Código do link" autoCapitalize="none" value={resetToken} onChangeText={setResetToken} />
        <TextInput style={s.input} placeholder="Nova senha (mín. 8)" secureTextEntry value={newPass} onChangeText={setNewPass} />
        {reset.isError && <ErrBanner text={errText(reset.error, "Link inválido ou expirado.")} />}
        <Btn
          label={reset.isPending ? "Salvando…" : "Redefinir senha"}
          disabled={reset.isPending || newPass.length < 8 || !resetToken.trim()}
          onPress={() => reset.mutate()}
        />
        <Link label="Voltar ao login" onPress={() => setScreen("auth")} />
      </Shell>
    );
  }

  // --- login / cadastro ---
  return (
    <Shell>
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
      {google.isError && <ErrBanner text={errText(google.error, "Não foi possível entrar com o Google.")} />}

      <View style={s.orRow}>
        <View style={s.orLine} />
        <Text style={s.orText}>ou</Text>
        <View style={s.orLine} />
      </View>

      <View style={s.seg}>
        {(["login", "register"] as const).map((mo) => (
          <Pressable key={mo} style={[s.segBtn, mode === mo && s.segOn]} onPress={() => setMode(mo)}>
            <Text style={[s.segText, mode === mo && s.segTextOn]}>{mo === "login" ? "Entrar" : "Criar conta"}</Text>
          </Pressable>
        ))}
      </View>

      <TextInput style={s.input} placeholder="E-mail" autoCapitalize="none" keyboardType="email-address" value={email} onChangeText={setEmail} />
      {mode === "register" && (
        <TextInput style={s.input} placeholder="Nome de usuário" autoCapitalize="none" value={username} onChangeText={setUsername} />
      )}
      <TextInput style={s.input} placeholder="Senha" secureTextEntry value={password} onChangeText={setPassword} />

      {m.isError && <ErrBanner text={errText(m.error, "Falha na autenticação.")} />}

      <Btn
        label={m.isPending ? "…" : mode === "login" ? "Entrar" : "Criar conta"}
        disabled={m.isPending}
        onPress={() => m.mutate()}
      />
      {mode === "login" && <Link label="Esqueci minha senha" onPress={() => { forgot.reset(); setScreen("forgot"); }} />}
    </Shell>
  );
}

function Shell({ children }: { children: React.ReactNode }) {
  return (
    <View style={{ flex: 1, backgroundColor: C.bg }}>
      <AuroraBg tint="coral" />
      <ScrollView contentContainerStyle={s.center} keyboardShouldPersistTaps="handled">
        {children}
      </ScrollView>
    </View>
  );
}

function ErrBanner({ text }: { text: string }) {
  return (
    <View style={s.errBanner}>
      <Ionicons name="alert-circle" size={16} color={C.crit} />
      <Text style={s.errText}>{text}</Text>
    </View>
  );
}

function Btn({ label, disabled, onPress }: { label: string; disabled?: boolean; onPress: () => void }) {
  return (
    <Pressable style={[s.btn, disabled && { opacity: 0.5 }]} disabled={disabled} onPress={onPress}>
      <Text style={s.btnText}>{label}</Text>
    </Pressable>
  );
}

function Link({ label, onPress }: { label: string; onPress: () => void }) {
  return (
    <Pressable onPress={onPress} hitSlop={8}>
      <Text style={s.link}>{label}</Text>
    </Pressable>
  );
}

const s = StyleSheet.create({
  center: { flexGrow: 1, alignItems: "center", justifyContent: "center", gap: 12, padding: 24 },
  logo: { width: 64, height: 64, borderRadius: 16, marginBottom: 4 },
  h1: { fontSize: 26, fontWeight: "800", color: C.ink, marginBottom: 2 },
  muted: { color: C.ink3, fontSize: 13, textAlign: "center", maxWidth: 300, lineHeight: 18 },
  gbtn: {
    width: 300,
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
  orRow: { flexDirection: "row", alignItems: "center", gap: 10, width: 300, marginVertical: 2 },
  orLine: { flex: 1, height: 1, backgroundColor: C.line },
  orText: { color: C.ink3, fontSize: 12 },
  seg: { flexDirection: "row", backgroundColor: C.sunken, borderRadius: 999, padding: 3, marginVertical: 6 },
  segBtn: { paddingVertical: 8, paddingHorizontal: 20, borderRadius: 999 },
  segOn: { backgroundColor: C.surface },
  segText: { color: C.ink3, fontWeight: "600" },
  segTextOn: { color: C.tealDeep },
  input: {
    width: 300,
    borderWidth: 1.5,
    borderColor: C.line,
    borderRadius: 8,
    paddingHorizontal: 12,
    paddingVertical: 11,
    backgroundColor: C.surface,
    color: C.ink,
  },
  errBanner: {
    width: 300,
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
    backgroundColor: "rgba(199,64,43,0.10)",
    borderWidth: 1,
    borderColor: "rgba(199,64,43,0.35)",
    borderRadius: 8,
    paddingVertical: 9,
    paddingHorizontal: 11,
  },
  errText: { color: C.crit, fontSize: 13, flex: 1, fontWeight: "600" },
  ok: { color: C.good, fontSize: 13, textAlign: "center", maxWidth: 300 },
  btn: { width: 300, paddingVertical: 13, borderRadius: 10, alignItems: "center", marginTop: 2, backgroundColor: C.coral },
  btnText: { color: "#fff", fontWeight: "800" },
  link: { color: C.teal, fontWeight: "700", fontSize: 13, marginTop: 4, padding: 4 },
});
