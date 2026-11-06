import { AppRouter } from "./router";
import { AuthProvider } from "./AuthContext";

export function App() {
  return (
    <AuthProvider>
      <AppRouter />
    </AuthProvider>
  );
}
