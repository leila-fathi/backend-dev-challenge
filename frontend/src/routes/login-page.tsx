import { useState } from "react";
import type { FormEvent } from "react";
import { useNavigate } from "@tanstack/react-router";
import {
  Button,
  Card,
  Flex,
  Heading,
  Separator,
  Text,
  TextField,
} from "@radix-ui/themes";
import { login } from "../api";
import { useAuthStore } from "../stores/auth-store";
import { useWorkspaceStore } from "../stores/workspace-store";

function toErrorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }
  return "Unexpected error";
}

export function LoginPage() {
  const navigate = useNavigate();
  const token = useAuthStore((state) => state.token);
  const setAuth = useAuthStore((state) => state.setAuth);
  const clearAuth = useAuthStore((state) => state.clearAuth);
  const status = useWorkspaceStore((state) => state.status);
  const setStatus = useWorkspaceStore((state) => state.setStatus);
  const clearWorkspace = useWorkspaceStore((state) => state.clearWorkspace);

  const [email, setEmail] = useState("owner@example.com");
  const [password, setPassword] = useState("password");
  const [isBusy, setIsBusy] = useState(false);
  const [error, setError] = useState("");

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setIsBusy(true);
    setError("");
    try {
      const auth = await login(email.trim(), password);
      setAuth(auth);
      setStatus(`Logged in as ${auth.displayName} (${auth.email}).`);
      await navigate({ to: "/dashboard" });
    } catch (loginError) {
      setError(toErrorMessage(loginError));
    } finally {
      setIsBusy(false);
    }
  }

  function handleResetSession() {
    clearAuth();
    clearWorkspace();
    setError("");
    setStatus("Not logged in.");
  }

  return (
    <Flex direction="column" gap="4">
      <Heading size="8">Hiring Challenge Frontend</Heading>
      <Text color="gray">
        Login with backend GraphQL and continue to the protected dashboard routes.
      </Text>

      <Card size="3">
        <Flex direction="column" gap="4">
          <Heading size="5">Login</Heading>
          <form onSubmit={(event) => void handleSubmit(event)}>
            <Flex direction="column" gap="3">
              <Text as="label" size="2">
                Email
              </Text>
              <TextField.Root
                type="email"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                placeholder="owner@example.com"
                required
              />

              <Text as="label" size="2">
                Password
              </Text>
              <TextField.Root
                type="password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                placeholder="password"
                required
              />

              <Flex gap="2" wrap="wrap" mt="2">
                <Button type="submit" disabled={isBusy}>
                  {isBusy ? "Logging in..." : "Login"}
                </Button>
                {token ? (
                  <>
                    <Button
                      type="button"
                      variant="soft"
                      onClick={() => void navigate({ to: "/dashboard" })}
                    >
                      Go To Dashboard
                    </Button>
                    <Button type="button" variant="outline" onClick={handleResetSession}>
                      Logout
                    </Button>
                  </>
                ) : null}
              </Flex>
            </Flex>
          </form>

          <Separator size="4" />
          <Text size="2" color="gray">
            {status}
          </Text>
          {error ? (
            <Text size="2" color="red">
              {error}
            </Text>
          ) : null}
        </Flex>
      </Card>
    </Flex>
  );
}
