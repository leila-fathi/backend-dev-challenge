import { useEffect, useMemo, useState } from "react";
import type { FormEvent } from "react";
import { Link, useNavigate } from "@tanstack/react-router";
import {
  Badge,
  Button,
  Card,
  Flex,
  Heading,
  Select,
  Separator,
  Text,
  TextField,
} from "@radix-ui/themes";
import {
  createProject,
  deleteProject,
  renameProject,
  renewToken,
  switchGroup,
} from "../api";
import type { Project } from "../api";
import { ConfirmDialog, TextInputDialog } from "../components/action-dialogs";
import { byName } from "../lib/tree";
import { useAuthStore } from "../stores/auth-store";
import { useWorkspaceStore } from "../stores/workspace-store";

function toErrorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }
  return "Unexpected error";
}

export function DashboardPage() {
  const navigate = useNavigate();

  const token = useAuthStore((state) => state.token);
  const activeGroupId = useAuthStore((state) => state.groupId);
  const email = useAuthStore((state) => state.email);
  const displayName = useAuthStore((state) => state.displayName);
  const setAuth = useAuthStore((state) => state.setAuth);
  const clearAuth = useAuthStore((state) => state.clearAuth);

  const workspace = useWorkspaceStore((state) => state.workspace);
  const selectedGroupId = useWorkspaceStore((state) => state.selectedGroupId);
  const isLoading = useWorkspaceStore((state) => state.isLoading);
  const storeError = useWorkspaceStore((state) => state.error);
  const status = useWorkspaceStore((state) => state.status);
  const setStatus = useWorkspaceStore((state) => state.setStatus);
  const setSelectedGroupId = useWorkspaceStore((state) => state.setSelectedGroupId);
  const refreshWorkspace = useWorkspaceStore((state) => state.refreshWorkspace);
  const clearWorkspace = useWorkspaceStore((state) => state.clearWorkspace);

  const [newProjectName, setNewProjectName] = useState("");
  const [isMutating, setIsMutating] = useState(false);
  const [localError, setLocalError] = useState("");
  const [renameTargetProject, setRenameTargetProject] = useState<Project | null>(null);
  const [deleteTargetProject, setDeleteTargetProject] = useState<Project | null>(null);
  const [renameProjectName, setRenameProjectName] = useState("");

  const projectsForSelectedGroup = useMemo(
    () =>
      workspace.projects
        .filter((project) => project.group_id === selectedGroupId)
        .sort(byName),
    [workspace.projects, selectedGroupId],
  );

  const canCreateForSelectedGroup = selectedGroupId !== "" && selectedGroupId === activeGroupId;

  useEffect(() => {
    void refreshWorkspace(token, activeGroupId);
  }, [activeGroupId, token, refreshWorkspace]);

  async function handleCreateProject(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!canCreateForSelectedGroup) {
      setLocalError(
        "Creating projects is restricted to your active token group (x-hasura-group-id).",
      );
      return;
    }

    const name = newProjectName.trim();
    if (!name) {
      setLocalError("Project name is required.");
      return;
    }

    setIsMutating(true);
    setLocalError("");
    try {
      await createProject(token, { name });
      setStatus(`Project "${name}" created.`);
      setNewProjectName("");
      await refreshWorkspace(token, selectedGroupId);
    } catch (error) {
      setLocalError(toErrorMessage(error));
    } finally {
      setIsMutating(false);
    }
  }

  async function confirmRenameProject(nextName: string): Promise<boolean> {
    if (!renameTargetProject) {
      return false;
    }
    if (nextName.trim() === renameTargetProject.name) {
      return true;
    }
    setIsMutating(true);
    setLocalError("");
    try {
      await renameProject(token, { id: renameTargetProject.id, name: nextName.trim() });
      setStatus(`Project renamed to "${nextName.trim()}".`);
      await refreshWorkspace(token, selectedGroupId);
      return true;
    } catch (error) {
      setLocalError(toErrorMessage(error));
      return false;
    } finally {
      setIsMutating(false);
    }
  }

  async function confirmDeleteProject(): Promise<boolean> {
    if (!deleteTargetProject) {
      return false;
    }
    setIsMutating(true);
    setLocalError("");
    try {
      await deleteProject(token, { id: deleteTargetProject.id });
      setStatus(`Project "${deleteTargetProject.name}" deleted.`);
      await refreshWorkspace(token, selectedGroupId);
      return true;
    } catch (error) {
      setLocalError(toErrorMessage(error));
      return false;
    } finally {
      setIsMutating(false);
    }
  }

  async function handleRenewToken() {
    setIsMutating(true);
    setLocalError("");
    try {
      const payload = await renewToken(token);
      setAuth(payload);
      setStatus(`Token renewed for ${payload.displayName} (${payload.email}).`);
      await refreshWorkspace(payload.token, payload.groupId);
    } catch (error) {
      setLocalError(toErrorMessage(error));
    } finally {
      setIsMutating(false);
    }
  }

  async function handleSelectGroup(nextGroupId: string) {
    if (!nextGroupId) {
      return;
    }
    if (nextGroupId === activeGroupId) {
      setSelectedGroupId(nextGroupId);
      return;
    }

    setIsMutating(true);
    setLocalError("");
    try {
      const payload = await switchGroup(token, nextGroupId);
      setAuth(payload);
      setSelectedGroupId(payload.groupId);
      setStatus(`Switched to group ${payload.groupId}.`);
      await refreshWorkspace(payload.token, payload.groupId);
    } catch (error) {
      setLocalError(toErrorMessage(error));
    } finally {
      setIsMutating(false);
    }
  }

  async function handleLogout() {
    clearAuth();
    clearWorkspace();
    setStatus("Not logged in.");
    await navigate({ to: "/login" });
  }

  const errorMessage = localError || storeError;

  return (
    <>
      <Flex direction="column" gap="4">
      <Flex justify="between" align="center" wrap="wrap" gap="3">
        <div>
          <Heading size="7">Dashboard</Heading>
          <Text color="gray">
            Signed in as {displayName || email} ({email})
          </Text>
        </div>
        <Flex gap="2" wrap="wrap">
          <Button variant="soft" onClick={() => void refreshWorkspace(token, activeGroupId)}>
            Refresh
          </Button>
          <Button variant="soft" onClick={() => void handleRenewToken()}>
            Renew Token
          </Button>
          <Button color="red" variant="outline" onClick={() => void handleLogout()}>
            Logout
          </Button>
        </Flex>
      </Flex>

      <Card size="3">
        <Flex direction="column" gap="3">
          <Heading size="5">Group Selector</Heading>
          <Select.Root value={selectedGroupId} onValueChange={(value) => void handleSelectGroup(value)}>
            <Select.Trigger placeholder="Select group" disabled={isMutating} />
            <Select.Content>
              {workspace.groups.map((group) => (
                <Select.Item key={group.id} value={group.id}>
                  {group.name}
                </Select.Item>
              ))}
            </Select.Content>
          </Select.Root>
          <Text size="2" color="gray">
            Current token group: <Badge variant="soft">{activeGroupId || "-"}</Badge>
          </Text>
        </Flex>
      </Card>

      <Card size="3">
        <Flex direction="column" gap="3">
          <Heading size="5">Projects</Heading>
          <form onSubmit={(event) => void handleCreateProject(event)}>
            <Flex gap="2" wrap="wrap" align="center">
              <TextField.Root
                placeholder="New project name"
                value={newProjectName}
                onChange={(event) => setNewProjectName(event.target.value)}
                style={{ minWidth: 260 }}
              />
              <Button type="submit" disabled={!canCreateForSelectedGroup || isMutating}>
                Create Project
              </Button>
            </Flex>
          </form>
          {!canCreateForSelectedGroup ? (
            <Text size="2" color="amber">
              Create is disabled because selected group does not match your token group.
            </Text>
          ) : null}
          <Separator size="4" />
          <Flex direction="column" gap="2">
            {projectsForSelectedGroup.map((project) => (
              <Card key={project.id} variant="surface">
                <Flex justify="between" align="center" wrap="wrap" gap="2">
                  <div>
                    <Text weight="medium">{project.name}</Text>
                    <Text as="div" size="1" color="gray">
                      {project.id}
                    </Text>
                  </div>
                  <Flex gap="2" wrap="wrap">
                    <Button asChild variant="solid">
                      <Link to="/projects/$projectId" params={{ projectId: project.id }}>
                        Open
                      </Link>
                    </Button>
                    <Button
                      variant="soft"
                      onClick={() => {
                        setRenameTargetProject(project);
                        setRenameProjectName(project.name);
                      }}
                      disabled={isMutating}
                    >
                      Rename
                    </Button>
                    <Button
                      color="red"
                      variant="outline"
                      onClick={() => setDeleteTargetProject(project)}
                      disabled={isMutating}
                    >
                      Delete
                    </Button>
                  </Flex>
                </Flex>
              </Card>
            ))}
            {projectsForSelectedGroup.length === 0 ? (
              <Text color="gray" size="2">
                No projects found for this group.
              </Text>
            ) : null}
          </Flex>
        </Flex>
      </Card>

      <Text size="2" color="gray">
        {isLoading ? "Loading workspace..." : status}
      </Text>
      {errorMessage ? (
        <Text size="2" color="red">
          {errorMessage}
        </Text>
      ) : null}
      </Flex>

      <TextInputDialog
        open={renameTargetProject !== null}
        title="Rename project"
        description={
          renameTargetProject
            ? `Set a new name for "${renameTargetProject.name}".`
            : undefined
        }
        label="Project name"
        confirmLabel="Rename"
        value={renameProjectName}
        onValueChange={setRenameProjectName}
        pending={isMutating}
        onOpenChange={(open) => {
          if (!open) {
            setRenameTargetProject(null);
          }
        }}
        onConfirm={confirmRenameProject}
      />

      <ConfirmDialog
        open={deleteTargetProject !== null}
        title="Delete project"
        description={
          deleteTargetProject
            ? `Delete "${deleteTargetProject.name}"? This removes all nested folders and images.`
            : "Delete selected project?"
        }
        confirmLabel="Delete"
        pending={isMutating}
        danger
        onOpenChange={(open) => {
          if (!open) {
            setDeleteTargetProject(null);
          }
        }}
        onConfirm={confirmDeleteProject}
      />
    </>
  );
}
