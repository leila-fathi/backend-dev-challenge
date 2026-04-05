import { Outlet } from "@tanstack/react-router";
import { Container, Theme } from "@radix-ui/themes";

export function RootLayout() {
  return (
    <Theme appearance="dark" accentColor="iris" grayColor="slate" radius="medium">
      <Container size="4" px="4" py="6" className="page-container">
        <Outlet />
      </Container>
    </Theme>
  );
}
