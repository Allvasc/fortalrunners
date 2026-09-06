import { useState } from "react";
import {
  ActivityIndicator,
  Alert,
  FlatList,
  StyleSheet,
  Text,
  TextInput,
  TouchableOpacity,
  View,
} from "react-native";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type Shoe } from "../lib/api";
import { C } from "../theme";
import { clock, km } from "../format";

// Gestão de tênis — quilometragem por par, custo por km, ciclo de vida (plano §4).
export function ShoesScreen() {
  const qc = useQueryClient();
  const shoes = useQuery({ queryKey: ["shoes"], queryFn: api.shoes });
  const [form, setForm] = useState(false);
  const [brand, setBrand] = useState("");
  const [model, setModel] = useState("");
  const [nick, setNick] = useState("");
  const [price, setPrice] = useState("");
  const [goalKm, setGoalKm] = useState("700");

  const create = useMutation({
    mutationFn: () =>
      api.createShoe({
        brand: brand.trim(),
        model: model.trim(),
        nickname: nick.trim() || undefined,
        purchase_price_cents: price ? Math.round(parseFloat(price.replace(",", ".")) * 100) : undefined,
        lifespan_goal_m: (parseInt(goalKm, 10) || 700) * 1000,
      }),
    onSuccess: () => {
      setForm(false);
      setBrand("");
      setModel("");
      setNick("");
      setPrice("");
      qc.invalidateQueries({ queryKey: ["shoes"] });
    },
    onError: (e: any) => Alert.alert("Ops", e.message || "Erro ao cadastrar."),
  });

  const retire = useMutation({
    mutationFn: (id: string) => api.retireShoe(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["shoes"] }),
  });

  const active = (shoes.data?.shoes ?? []).filter((s) => s.status !== "retired");
  const retired = (shoes.data?.shoes ?? []).filter((s) => s.status === "retired");

  return (
    <View style={s.wrap}>
      <View style={s.head}>
        <View style={{ flexDirection: "row", justifyContent: "space-between", alignItems: "center" }}>
          <Text style={s.title}>Meus tênis</Text>
          <TouchableOpacity style={s.newBtn} onPress={() => setForm((v) => !v)}>
            <Text style={s.newTxt}>{form ? "Cancelar" : "+ Novo par"}</Text>
          </TouchableOpacity>
        </View>
        <Text style={s.sub}>Quilometragem, custo por km e vida útil de cada par.</Text>
      </View>

      <FlatList
        data={active}
        keyExtractor={(x) => x.id}
        contentContainerStyle={s.list}
        ListHeaderComponent={
          form ? (
            <View style={s.form}>
              <TextInput style={s.input} placeholder="Marca (ex: Nike)" placeholderTextColor={C.ink3} value={brand} onChangeText={setBrand} />
              <TextInput style={s.input} placeholder="Modelo (ex: Pegasus 41)" placeholderTextColor={C.ink3} value={model} onChangeText={setModel} />
              <TextInput style={s.input} placeholder="Apelido (opcional)" placeholderTextColor={C.ink3} value={nick} onChangeText={setNick} />
              <View style={{ flexDirection: "row", gap: 8 }}>
                <TextInput style={[s.input, { flex: 1 }]} placeholder="Preço R$" placeholderTextColor={C.ink3} keyboardType="numeric" value={price} onChangeText={setPrice} />
                <TextInput style={[s.input, { flex: 1 }]} placeholder="Meta km" placeholderTextColor={C.ink3} keyboardType="numeric" value={goalKm} onChangeText={setGoalKm} />
              </View>
              <TouchableOpacity
                style={[s.save, (!brand.trim() || !model.trim() || create.isPending) && { opacity: 0.5 }]}
                disabled={!brand.trim() || !model.trim() || create.isPending}
                onPress={() => create.mutate()}
              >
                <Text style={s.saveTxt}>{create.isPending ? "Salvando…" : "Cadastrar par"}</Text>
              </TouchableOpacity>
            </View>
          ) : null
        }
        ListEmptyComponent={
          shoes.isLoading ? <ActivityIndicator color={C.teal} style={{ marginTop: 16 }} /> : (
            <Text style={s.muted}>Nenhum par cadastrado. Adicione o seu primeiro.</Text>
          )
        }
        renderItem={({ item }) => <ShoeCard shoe={item} onRetire={() => retire.mutate(item.id)} />}
        ListFooterComponent={
          retired.length > 0 ? (
            <>
              <Text style={s.section}>Aposentados</Text>
              {retired.map((r) => (
                <ShoeCard key={r.id} shoe={r} onRetire={() => {}} />
              ))}
            </>
          ) : null
        }
      />
    </View>
  );
}

function ShoeCard({ shoe, onRetire }: { shoe: Shoe; onRetire: () => void }) {
  const pct = Math.min(100, Math.round(shoe.life_pct));
  const barColor = pct >= 100 ? C.crit : pct >= 75 ? C.gold : C.teal;
  const costTxt =
    shoe.cost_per_km_cents != null ? `R$ ${(shoe.cost_per_km_cents / 100).toFixed(2)}/km` : "—";
  return (
    <View style={s.card}>
      <View style={{ flexDirection: "row", justifyContent: "space-between", alignItems: "flex-start" }}>
        <View style={{ flex: 1 }}>
          <Text style={s.shoeName}>
            {shoe.nickname || `${shoe.brand} ${shoe.model}`}
          </Text>
          {shoe.nickname ? <Text style={s.muted}>{shoe.brand} {shoe.model}</Text> : null}
        </View>
        {shoe.status !== "retired" && (
          <TouchableOpacity onPress={onRetire}>
            <Text style={s.retireLink}>aposentar</Text>
          </TouchableOpacity>
        )}
      </View>

      <View style={s.track}>
        <View style={[s.fill, { width: `${pct}%`, backgroundColor: barColor }]} />
      </View>
      <Text style={s.lifeTxt}>
        {km(shoe.total_distance_m)} / {km(shoe.lifespan_goal_m)} km · {pct}% da vida útil
      </Text>

      <View style={s.figs}>
        <Fig v={String(shoe.run_count)} l="corridas" />
        <Fig v={clock(shoe.total_moving_s)} l="em movimento" />
        <Fig v={costTxt} l="custo/km" />
      </View>
      {shoe.alert === "trocar_em_breve" && <Text style={[s.alert, { color: C.gold }]}>⚠ planeje a troca</Text>}
      {shoe.alert === "vencido" && <Text style={[s.alert, { color: C.crit }]}>⚠ amortecimento gasto — risco de lesão</Text>}
    </View>
  );
}

function Fig({ v, l }: { v: string; l: string }) {
  return (
    <View style={{ alignItems: "center", flex: 1 }}>
      <Text style={s.figV}>{v}</Text>
      <Text style={s.figL}>{l}</Text>
    </View>
  );
}

const s = StyleSheet.create({
  wrap: { flex: 1, backgroundColor: C.bg },
  head: { padding: 16, borderBottomWidth: 1, borderBottomColor: C.line },
  title: { fontSize: 20, fontWeight: "800", color: C.ink },
  sub: { fontSize: 13, color: C.ink3, marginTop: 2 },
  newBtn: { backgroundColor: C.teal, borderRadius: 8, paddingHorizontal: 12, paddingVertical: 6 },
  newTxt: { color: "#fff", fontWeight: "700", fontSize: 12 },
  list: { padding: 16, gap: 12, paddingBottom: 40 },
  form: { backgroundColor: C.surface, borderRadius: 12, borderWidth: 1, borderColor: C.line, padding: 14, gap: 10, marginBottom: 4 },
  input: {
    borderWidth: 1,
    borderColor: C.line,
    borderRadius: 8,
    paddingHorizontal: 10,
    paddingVertical: 9,
    color: C.ink,
    backgroundColor: C.bg,
  },
  save: { backgroundColor: C.teal, borderRadius: 8, paddingVertical: 10, alignItems: "center" },
  saveTxt: { color: "#fff", fontWeight: "700" },
  card: { backgroundColor: C.surface, borderRadius: 12, borderWidth: 1, borderColor: C.line, padding: 14, gap: 8 },
  shoeName: { fontSize: 15, fontWeight: "700", color: C.ink },
  muted: { color: C.ink3, fontSize: 12 },
  retireLink: { color: C.coral, fontSize: 12, fontWeight: "700" },
  track: { height: 8, backgroundColor: C.line, borderRadius: 4, overflow: "hidden", marginTop: 4 },
  fill: { height: "100%" },
  lifeTxt: { fontSize: 12, color: C.ink2 },
  figs: { flexDirection: "row", marginTop: 6, borderTopWidth: 1, borderTopColor: C.line, paddingTop: 8 },
  figV: { fontSize: 14, fontWeight: "800", color: C.ink },
  figL: { fontSize: 10, textTransform: "uppercase", color: C.ink3, marginTop: 1 },
  alert: { fontSize: 12, fontWeight: "700", marginTop: 4 },
  section: { fontWeight: "800", color: C.ink, fontSize: 15, marginTop: 14, marginBottom: 6 },
});
