import "react-native-gesture-handler";
import { useEffect, useState } from "react";
import { Image, StyleSheet, Text, TouchableOpacity, View } from "react-native";
import { GestureHandlerRootView } from "react-native-gesture-handler";
import { StatusBar } from "expo-status-bar";
import * as SplashScreen from "expo-splash-screen";
import * as WebBrowser from "expo-web-browser";
import { NavigationContainer, DefaultTheme, useNavigation } from "@react-navigation/native";
import { createNativeStackNavigator } from "@react-navigation/native-stack";
import {
  createDrawerNavigator,
  DrawerContentScrollView,
  DrawerItem,
  type DrawerContentComponentProps,
} from "@react-navigation/drawer";
import { Ionicons } from "@expo/vector-icons";
import { QueryClient, QueryClientProvider, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, isAuthed } from "./src/lib/api";
import { C } from "./src/theme";
import { cleanupStaleRecording } from "./src/lib/recorder";
import { Splash } from "./src/screens/Splash";
import { Auth } from "./src/screens/Auth";
import { Home } from "./src/screens/Home";
import { Map } from "./src/screens/Map";
import { Recording } from "./src/screens/Recording";
import { Summary } from "./src/screens/Summary";
import { Landmarks } from "./src/screens/Landmarks";
import { RoutesScreen } from "./src/screens/Routes";
import { SocialScreen } from "./src/screens/Social";
import { ClubsScreen } from "./src/screens/Clubs";
import { EventsScreen } from "./src/screens/Events";
import { AICoachScreen } from "./src/screens/AICoach";
import { SafetyScreen } from "./src/screens/Safety";
import { SettingsScreen } from "./src/screens/Settings";
import { RankingScreen } from "./src/screens/Ranking";
import { ChallengesScreen } from "./src/screens/Challenges";
import { ProfileScreen } from "./src/screens/Profile";
import { HistoryScreen } from "./src/screens/History";
import "./src/lib/recorder"; // registra a task de background no import

WebBrowser.maybeCompleteAuthSession();

export type RootStack = {
  Drawer: undefined;
  Home: undefined;
  Map: undefined;
  Ranking: undefined;
  Challenges: undefined;
  Profile: undefined;
  Recording: undefined;
  Summary: { runId: string };
  Landmarks: undefined;
  Routes: undefined;
  Social: undefined;
  Clubs: undefined;
  Events: undefined;
  AICoach: undefined;
  Safety: undefined;
  Settings: undefined;
  History: undefined;
};

const Stack = createNativeStackNavigator<RootStack>();
const Drawer = createDrawerNavigator<RootStack>();
const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });

SplashScreen.preventAutoHideAsync().catch(() => {});
SplashScreen.setOptions?.({ fade: true, duration: 300 });

const navTheme = {
  ...DefaultTheme,
  colors: { ...DefaultTheme.colors, background: C.bg, card: C.bg, text: C.ink, border: C.line, primary: C.teal },
};

export default function App() {
  return (
    <GestureHandlerRootView style={{ flex: 1 }}>
      <QueryClientProvider client={qc}>
        <StatusBar style="dark" />
        <Root />
      </QueryClientProvider>
    </GestureHandlerRootView>
  );
}

function Root() {
  const authed = useQuery({ queryKey: ["authed"], queryFn: isAuthed });
  const [minTime, setMinTime] = useState(false); // segura a splash cheia por ~1.6s

  useEffect(() => {
    cleanupStaleRecording();
    SplashScreen.hideAsync().catch(() => {});
    const id = setTimeout(() => setMinTime(true), 1600);
    return () => clearTimeout(id);
  }, []);

  if (authed.isLoading || !minTime) return <Splash />;
  if (!authed.data) return <Auth />;

  return (
    <NavigationContainer theme={navTheme}>
      <Stack.Navigator
        screenOptions={{
          headerShown: false,
          contentStyle: { backgroundColor: C.bg },
          animation: "slide_from_right",
        }}
      >
        <Stack.Screen name="Drawer" component={DrawerNav} />
        <Stack.Screen
          name="Recording"
          component={Recording}
          options={{
            headerShown: true,
            title: "Correndo",
            headerStyle: { backgroundColor: C.bg },
            headerTintColor: C.ink,
            headerShadowVisible: false,
            gestureEnabled: false,
            animation: "slide_from_bottom",
          }}
        />
        <Stack.Screen
          name="Summary"
          component={Summary}
          options={{
            headerShown: true,
            title: "Resumo",
            headerStyle: { backgroundColor: C.bg },
            headerTintColor: C.ink,
            headerShadowVisible: false,
            headerBackVisible: false,
            animation: "fade",
          }}
        />
      </Stack.Navigator>
    </NavigationContainer>
  );
}

const NAV: { name: keyof RootStack; label: string; icon: keyof typeof Ionicons.glyphMap; comp: React.ComponentType<any> }[] = [
  { name: "Home", label: "Início", icon: "home-outline", comp: Home },
  { name: "Map", label: "Mapa", icon: "map-outline", comp: Map },
  { name: "Ranking", label: "Ranking", icon: "trophy-outline", comp: RankingScreen },
  { name: "Challenges", label: "Desafios", icon: "flame-outline", comp: ChallengesScreen },
  { name: "History", label: "Histórico", icon: "time-outline", comp: HistoryScreen },
  { name: "Landmarks", label: "Marcos & Selos", icon: "ribbon-outline", comp: Landmarks },
  { name: "Routes", label: "Rotas", icon: "trail-sign-outline", comp: RoutesScreen },
  { name: "Social", label: "Comunidade", icon: "people-outline", comp: SocialScreen },
  { name: "Clubs", label: "Clubes", icon: "shirt-outline", comp: ClubsScreen },
  { name: "Events", label: "Eventos & QR", icon: "qr-code-outline", comp: EventsScreen },
  { name: "AICoach", label: "Coach IA", icon: "sparkles-outline", comp: AICoachScreen },
  { name: "Safety", label: "Segurança & SOS", icon: "shield-checkmark-outline", comp: SafetyScreen },
  { name: "Profile", label: "Perfil", icon: "person-outline", comp: ProfileScreen },
  { name: "Settings", label: "Ajustes", icon: "settings-outline", comp: SettingsScreen },
];

function DrawerNav() {
  return (
    <Drawer.Navigator
      drawerContent={(p) => <DrawerBody {...p} />}
      screenOptions={{
        headerStyle: { backgroundColor: C.bg },
        headerTintColor: C.ink,
        headerShadowVisible: false,
        headerTitleStyle: { fontWeight: "800" },
        drawerActiveTintColor: C.teal,
        drawerInactiveTintColor: C.ink2,
        drawerActiveBackgroundColor: C.sunken,
        sceneStyle: { backgroundColor: C.bg },
      }}
    >
      {NAV.map((n) => (
        <Drawer.Screen key={n.name} name={n.name} component={n.comp} options={{ title: n.label }} />
      ))}
    </Drawer.Navigator>
  );
}

function DrawerBody(props: DrawerContentComponentProps) {
  const qcli = useQueryClient();
  const me = useQuery({ queryKey: ["me"], queryFn: api.me });
  const active = props.state.routeNames[props.state.index];

  return (
    <View style={{ flex: 1, backgroundColor: C.bg }}>
      <View style={d.head}>
        <Image source={require("./assets/icon.png")} style={d.logo} />
        <Text style={d.name} numberOfLines={1}>
          {me.data?.username ?? "corredor"}
        </Text>
        <Text style={d.sub}>{me.data?.athlete_id}</Text>
      </View>

      <DrawerContentScrollView {...props} contentContainerStyle={{ paddingTop: 4 }}>
        {NAV.map((n) => (
          <DrawerItem
            key={n.name}
            label={n.label}
            focused={active === n.name}
            activeTintColor={C.teal}
            inactiveTintColor={C.ink2}
            activeBackgroundColor={C.sunken}
            icon={({ color, size }) => <Ionicons name={n.icon} size={size} color={color} />}
            onPress={() => props.navigation.navigate(n.name as never)}
          />
        ))}
      </DrawerContentScrollView>

      <TouchableOpacity
        style={d.logout}
        onPress={async () => {
          try {
            await api.logout();
          } catch {}
          qcli.clear();
          qcli.setQueryData(["authed"], false);
        }}
      >
        <Ionicons name="log-out-outline" size={20} color={C.crit} />
        <Text style={d.logoutTxt}>Sair</Text>
      </TouchableOpacity>
    </View>
  );
}

/** Botão de hambúrguer para telas que precisam abrir o drawer manualmente. */
export function MenuButton() {
  const nav = useNavigation<any>();
  return (
    <TouchableOpacity onPress={() => nav.openDrawer?.()} hitSlop={12} style={{ paddingHorizontal: 4 }}>
      <Ionicons name="menu" size={26} color={C.ink} />
    </TouchableOpacity>
  );
}

const d = StyleSheet.create({
  head: { paddingTop: 54, paddingHorizontal: 18, paddingBottom: 14, borderBottomWidth: 1, borderBottomColor: C.line },
  logo: { width: 44, height: 44, borderRadius: 12, marginBottom: 8 },
  name: { fontSize: 17, fontWeight: "800", color: C.ink },
  sub: { fontSize: 12, color: C.ink3, marginTop: 1 },
  logout: {
    flexDirection: "row",
    alignItems: "center",
    gap: 10,
    padding: 16,
    borderTopWidth: 1,
    borderTopColor: C.line,
  },
  logoutTxt: { color: C.crit, fontWeight: "700" },
});
