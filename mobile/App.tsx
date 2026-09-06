import { useCallback } from "react";
import { View } from "react-native";
import { StatusBar } from "expo-status-bar";
import * as SplashScreen from "expo-splash-screen";
import { NavigationContainer, DefaultTheme } from "@react-navigation/native";
import { createNativeStackNavigator } from "@react-navigation/native-stack";
import { createBottomTabNavigator } from "@react-navigation/bottom-tabs";
import { Ionicons } from "@expo/vector-icons";
import { QueryClient, QueryClientProvider, useQuery } from "@tanstack/react-query";
import { isAuthed } from "./src/lib/api";
import { C } from "./src/theme";
import { Boot } from "./src/screens/Boot";
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

// Um único param list — tabs e telas empilhadas compartilham os nomes.
export type RootStack = {
  Tabs: undefined;
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
const Tab = createBottomTabNavigator<RootStack>();
const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });

SplashScreen.preventAutoHideAsync().catch(() => {});

const navTheme = {
  ...DefaultTheme,
  colors: { ...DefaultTheme.colors, background: C.bg, card: C.bg, text: C.ink, border: C.line, primary: C.teal },
};

export default function App() {
  return (
    <QueryClientProvider client={qc}>
      <StatusBar style="dark" />
      <Root />
    </QueryClientProvider>
  );
}

function Root() {
  const authed = useQuery({ queryKey: ["authed"], queryFn: isAuthed });

  const onReady = useCallback(() => {
    SplashScreen.hideAsync().catch(() => {});
  }, []);

  if (authed.isLoading) return <Boot />;
  if (!authed.data) {
    return (
      <View style={{ flex: 1 }} onLayout={onReady}>
        <Auth />
      </View>
    );
  }

  return (
    <NavigationContainer theme={navTheme} onReady={onReady}>
      <Stack.Navigator
        screenOptions={{
          headerStyle: { backgroundColor: C.bg },
          headerTintColor: C.ink,
          headerShadowVisible: false,
          headerTitleStyle: { fontWeight: "800" },
          contentStyle: { backgroundColor: C.bg },
          animation: "slide_from_right",
        }}
      >
        <Stack.Screen name="Tabs" component={Tabs} options={{ headerShown: false }} />
        <Stack.Screen name="Landmarks" component={Landmarks} options={{ title: "Marcos & Selos" }} />
        <Stack.Screen name="Routes" component={RoutesScreen} options={{ title: "Rotas" }} />
        <Stack.Screen name="Social" component={SocialScreen} options={{ title: "Comunidade" }} />
        <Stack.Screen name="Clubs" component={ClubsScreen} options={{ title: "Clubes" }} />
        <Stack.Screen name="Events" component={EventsScreen} options={{ title: "Eventos & QR" }} />
        <Stack.Screen name="AICoach" component={AICoachScreen} options={{ title: "Coach IA" }} />
        <Stack.Screen name="Safety" component={SafetyScreen} options={{ title: "Segurança & SOS" }} />
        <Stack.Screen name="Settings" component={SettingsScreen} options={{ title: "Ajustes" }} />
        <Stack.Screen name="History" component={HistoryScreen} options={{ title: "Histórico" }} />
        <Stack.Screen
          name="Recording"
          component={Recording}
          options={{ title: "Correndo", gestureEnabled: false, animation: "slide_from_bottom" }}
        />
        <Stack.Screen
          name="Summary"
          component={Summary}
          options={{ title: "Resumo", headerBackVisible: false, animation: "fade" }}
        />
      </Stack.Navigator>
    </NavigationContainer>
  );
}

const ICON: Record<string, keyof typeof Ionicons.glyphMap> = {
  Home: "home",
  Map: "map",
  Ranking: "trophy",
  Challenges: "flame",
  Profile: "person",
};

function Tabs() {
  return (
    <Tab.Navigator
      screenOptions={({ route }) => ({
        headerShown: false,
        tabBarActiveTintColor: C.teal,
        tabBarInactiveTintColor: C.ink3,
        tabBarStyle: {
          backgroundColor: C.surface,
          borderTopColor: C.line,
          height: 60,
          paddingBottom: 6,
          paddingTop: 6,
        },
        tabBarLabelStyle: { fontSize: 11, fontWeight: "700" },
        tabBarIcon: ({ color, size, focused }) => (
          <Ionicons
            name={focused ? ICON[route.name] : (`${ICON[route.name]}-outline` as keyof typeof Ionicons.glyphMap)}
            size={size - 2}
            color={color}
          />
        ),
      })}
    >
      <Tab.Screen name="Home" component={Home} options={{ title: "Início" }} />
      <Tab.Screen name="Map" component={Map} options={{ title: "Mapa" }} />
      <Tab.Screen name="Ranking" component={RankingScreen} options={{ title: "Ranking" }} />
      <Tab.Screen name="Challenges" component={ChallengesScreen} options={{ title: "Desafios" }} />
      <Tab.Screen name="Profile" component={ProfileScreen} options={{ title: "Perfil" }} />
    </Tab.Navigator>
  );
}
