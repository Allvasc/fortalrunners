import { ActivityIndicator, View } from "react-native";
import { StatusBar } from "expo-status-bar";
import { NavigationContainer } from "@react-navigation/native";
import { createNativeStackNavigator } from "@react-navigation/native-stack";
import { QueryClient, QueryClientProvider, useQuery } from "@tanstack/react-query";
import { isAuthed } from "./src/lib/api";
import { C } from "./src/theme";
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

export type RootStack = {
  Home: undefined;
  Map: undefined;
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
  Ranking: undefined;
  Challenges: undefined;
  Profile: undefined;
  History: undefined;
};

const Stack = createNativeStackNavigator<RootStack>();
const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });

export default function App() {
  return (
    <QueryClientProvider client={qc}>
      <StatusBar style="auto" />
      <Root />
    </QueryClientProvider>
  );
}

function Root() {
  const authed = useQuery({ queryKey: ["authed"], queryFn: isAuthed });

  if (authed.isLoading) {
    return (
      <View style={{ flex: 1, alignItems: "center", justifyContent: "center", backgroundColor: C.bg }}>
        <ActivityIndicator />
      </View>
    );
  }
  if (!authed.data) return <Auth />;

  return (
    <NavigationContainer>
      <Stack.Navigator
        screenOptions={{
          headerStyle: { backgroundColor: C.bg },
          headerTintColor: C.ink,
          headerShadowVisible: false,
          contentStyle: { backgroundColor: C.bg },
        }}
      >
        <Stack.Screen name="Home" component={Home} options={{ title: "FortalRunners", headerShown: false }} />
        <Stack.Screen name="Map" component={Map} options={{ title: "Meu mapa" }} />
        <Stack.Screen name="Ranking" component={RankingScreen} options={{ title: "Ranking" }} />
        <Stack.Screen name="Challenges" component={ChallengesScreen} options={{ title: "Desafios" }} />
        <Stack.Screen name="History" component={HistoryScreen} options={{ title: "Histórico" }} />
        <Stack.Screen name="Profile" component={ProfileScreen} options={{ title: "Perfil" }} />
        <Stack.Screen name="Landmarks" component={Landmarks} options={{ title: "Marcos & Selos" }} />
        <Stack.Screen name="Routes" component={RoutesScreen} options={{ title: "Rotas" }} />
        <Stack.Screen name="Social" component={SocialScreen} options={{ title: "Feed Social" }} />
        <Stack.Screen name="Clubs" component={ClubsScreen} options={{ title: "Clubes" }} />
        <Stack.Screen name="Events" component={EventsScreen} options={{ title: "Eventos & QR" }} />
        <Stack.Screen name="AICoach" component={AICoachScreen} options={{ title: "Coach IA" }} />
        <Stack.Screen name="Safety" component={SafetyScreen} options={{ title: "Segurança & SOS" }} />
        <Stack.Screen name="Settings" component={SettingsScreen} options={{ title: "Ajustes" }} />
        <Stack.Screen name="Recording" component={Recording} options={{ title: "Correndo", gestureEnabled: false }} />
        <Stack.Screen name="Summary" component={Summary} options={{ title: "Resumo", headerBackVisible: false }} />
      </Stack.Navigator>
    </NavigationContainer>
  );
}
