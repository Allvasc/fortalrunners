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
import "./src/lib/recorder"; // registra a task de background no import

export type RootStack = {
  Home: undefined;
  Map: undefined;
  Recording: undefined;
  Summary: { runId: string };
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
        <Stack.Screen name="Recording" component={Recording} options={{ title: "Correndo", gestureEnabled: false }} />
        <Stack.Screen name="Summary" component={Summary} options={{ title: "Resumo", headerBackVisible: false }} />
      </Stack.Navigator>
    </NavigationContainer>
  );
}
