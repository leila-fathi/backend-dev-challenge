import { createRootRoute, createRoute, createRouter, redirect } from "@tanstack/react-router";
import { Card, Flex, Heading, Text } from "@radix-ui/themes";
import { DashboardPage } from "./routes/dashboard-page";
import { LoginPage } from "./routes/login-page";
import { ProjectPage } from "./routes/project-page";
import { RootLayout } from "./routes/root-layout";
import { useAuthStore } from "./stores/auth-store";

const rootRoute = createRootRoute({
  component: RootLayout,
  errorComponent: ({ error }) => (
    <Card size="3">
      <Flex direction="column" gap="2">
        <Heading size="5">Something went wrong</Heading>
        <Text color="red" size="2">
          {error instanceof Error ? error.message : "Unknown route error"}
        </Text>
      </Flex>
    </Card>
  ),
});

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  beforeLoad: () => {
    const token = useAuthStore.getState().token;
    throw redirect({ to: token ? "/dashboard" : "/login" });
  },
});

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/login",
  beforeLoad: () => {
    const token = useAuthStore.getState().token;
    if (token) {
      throw redirect({ to: "/dashboard" });
    }
  },
  component: LoginPage,
});

const dashboardRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/dashboard",
  beforeLoad: () => {
    const token = useAuthStore.getState().token;
    if (!token) {
      throw redirect({ to: "/login" });
    }
  },
  component: DashboardPage,
});

const projectRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/projects/$projectId",
  beforeLoad: () => {
    const token = useAuthStore.getState().token;
    if (!token) {
      throw redirect({ to: "/login" });
    }
  },
  component: ProjectPage,
});

const routeTree = rootRoute.addChildren([indexRoute, loginRoute, dashboardRoute, projectRoute]);

export const router = createRouter({
  routeTree,
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
