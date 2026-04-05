import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate, useParams } from "@tanstack/react-router";
import {
  Badge,
  Button,
  Card,
  Flex,
  Heading,
  Separator,
  Text,
} from "@radix-ui/themes";
import { createFolder, deleteNode, renameNode, uploadImage } from "../api";
import type { Folder, Image } from "../api";
import {
  ConfirmDialog,
  TextInputDialog,
  UploadImageDialog,
} from "../components/action-dialogs";
import { FileExplorerTree } from "../components/file-explorer-tree";
import { byName } from "../lib/tree";
import { useAuthStore } from "../stores/auth-store";
import { useWorkspaceStore } from "../stores/workspace-store";

function toErrorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }
  return "Unexpected error";
}

type NodeTarget = {
  id: string;
  name: string;
  kind: "folder" | "image";
};

export function ProjectPage() {
  const { projectId } = useParams({ from: "/projects/$projectId" });
  const navigate = useNavigate();

  const token = useAuthStore((state) => state.token);
  const activeGroupId = useAuthStore((state) => state.groupId);

  const workspace = useWorkspaceStore((state) => state.workspace);
  const refreshWorkspace = useWorkspaceStore((state) => state.refreshWorkspace);
  const status = useWorkspaceStore((state) => state.status);
  const setStatus = useWorkspaceStore((state) => state.setStatus);
  const storeError = useWorkspaceStore((state) => state.error);
  const isLoading = useWorkspaceStore((state) => state.isLoading);

  const [isMutating, setIsMutating] = useState(false);
  const [localError, setLocalError] = useState("");
  const [currentParentId, setCurrentParentId] = useState<string | null>(null);
  const [isCreateFolderOpen, setIsCreateFolderOpen] = useState(false);
  const [createFolderParentId, setCreateFolderParentId] = useState<string | null>(null);
  const [isUploadImageOpen, setIsUploadImageOpen] = useState(false);
  const [uploadImageParentId, setUploadImageParentId] = useState<string | null>(null);
  const [createFolderName, setCreateFolderName] = useState("");
  const [uploadImageName, setUploadImageName] = useState("image");
  const [uploadImageFile, setUploadImageFile] = useState<File | null>(null);
  const [renameTargetNode, setRenameTargetNode] = useState<NodeTarget | null>(null);
  const [renameNodeName, setRenameNodeName] = useState("");
  const [deleteTargetNode, setDeleteTargetNode] = useState<NodeTarget | null>(null);

  useEffect(() => {
    void refreshWorkspace(token, activeGroupId);
  }, [activeGroupId, token, refreshWorkspace]);

  const project = useMemo(
    () => workspace.projects.find((item) => item.id === projectId) ?? null,
    [projectId, workspace.projects],
  );

  const folders = useMemo(
    () =>
      workspace.folders
        .filter((folder) => folder.project_id === projectId)
        .sort(byName),
    [projectId, workspace.folders],
  );

  const images = useMemo(
    () =>
      workspace.images
        .filter((image) => image.project_id === projectId)
        .sort(byName),
    [projectId, workspace.images],
  );

  const canInsertFolder = project ? project.group_id === activeGroupId : false;
  const folderById = useMemo(
    () => new Map(folders.map((folder) => [folder.id, folder])),
    [folders],
  );
  const currentParentFolder = useMemo(
    () => (currentParentId ? folderById.get(currentParentId) ?? null : null),
    [currentParentId, folderById],
  );
  const currentFolders = useMemo(
    () =>
      folders
        .filter((folder) => folder.parent_id === currentParentId)
        .sort(byName),
    [currentParentId, folders],
  );
  const currentImages = useMemo(
    () =>
      images
        .filter((image) => image.parent_id === currentParentId)
        .sort(byName),
    [currentParentId, images],
  );
  const breadcrumbFolders = useMemo(() => {
    const path: Folder[] = [];
    const seen = new Set<string>();
    let cursorId: string | null = currentParentId;
    while (cursorId) {
      if (seen.has(cursorId)) {
        break;
      }
      seen.add(cursorId);
      const folder = folderById.get(cursorId);
      if (!folder) {
        break;
      }
      path.unshift(folder);
      cursorId = folder.parent_id;
    }
    return path;
  }, [currentParentId, folderById]);

  useEffect(() => {
    if (currentParentId && !folderById.has(currentParentId)) {
      setCurrentParentId(null);
    }
  }, [currentParentId, folderById]);

  function openCreateFolderDialog(parentId: string | null) {
    setCreateFolderParentId(parentId);
    setCreateFolderName("");
    setIsCreateFolderOpen(true);
  }

  async function confirmCreateFolder(name: string): Promise<boolean> {
    if (!project) {
      setLocalError("Project not found.");
      return false;
    }
    if (!canInsertFolder) {
      setLocalError(
        "Folder create is restricted to your token group (x-hasura-group-id).",
      );
      return false;
    }

    setIsMutating(true);
    setLocalError("");
    try {
      await createFolder(token, {
        projectId: project.id,
        parentId: createFolderParentId,
        name: name,
      });
      setStatus(`Folder "${name}" created.`);
      await refreshWorkspace(token, project.group_id);
      return true;
    } catch (error) {
      setLocalError(toErrorMessage(error));
      return false;
    } finally {
      setIsMutating(false);
    }
  }

  function openUploadImageDialog(parentId: string | null) {
    setUploadImageParentId(parentId);
    setUploadImageName("image");
    setUploadImageFile(null);
    setIsUploadImageOpen(true);
  }

  async function confirmUploadImage(payload: {
    name: string;
    file: File;
  }): Promise<boolean> {
    if (!project) {
      setLocalError("Project not found.");
      return false;
    }

    setIsMutating(true);
    setLocalError("");
    try {
      await uploadImage(token, {
        projectId: project.id,
        parentId: uploadImageParentId,
        name: payload.name,
        file: payload.file,
      });
      setStatus(`Image "${payload.name}" uploaded.`);
      await refreshWorkspace(token, project.group_id);
      return true;
    } catch (error) {
      setLocalError(toErrorMessage(error));
      return false;
    } finally {
      setIsMutating(false);
    }
  }

  async function confirmRenameNode(nextName: string): Promise<boolean> {
    if (!renameTargetNode) {
      return false;
    }
    if (nextName.trim() === renameTargetNode.name) {
      return true;
    }

    setIsMutating(true);
    setLocalError("");
    try {
      await renameNode(token, { id: renameTargetNode.id, name: nextName.trim() });
      setStatus(`${renameTargetNode.kind} renamed to "${nextName.trim()}".`);
      await refreshWorkspace(token, project?.group_id);
      return true;
    } catch (error) {
      setLocalError(toErrorMessage(error));
      return false;
    } finally {
      setIsMutating(false);
    }
  }

  async function confirmDeleteNode(): Promise<boolean> {
    if (!deleteTargetNode) {
      return false;
    }
    setIsMutating(true);
    setLocalError("");
    try {
      await deleteNode(token, { id: deleteTargetNode.id });
      setStatus(`${deleteTargetNode.kind} "${deleteTargetNode.name}" deleted.`);
      await refreshWorkspace(token, project?.group_id);
      return true;
    } catch (error) {
      setLocalError(toErrorMessage(error));
      return false;
    } finally {
      setIsMutating(false);
    }
  }

  const errorMessage = localError || storeError;

  if (!project && !isLoading) {
    return (
      <Card size="3">
        <Flex direction="column" gap="3">
          <Heading size="5">Project Not Found</Heading>
          <Text color="gray">
            Could not locate project <code>{projectId}</code> in your current workspace.
          </Text>
          <Button asChild>
            <Link to="/dashboard">Back To Dashboard</Link>
          </Button>
        </Flex>
      </Card>
    );
  }

  return (
    <>
      <Flex direction="column" gap="4">
      <Flex justify="between" align="center" wrap="wrap" gap="2">
        <div>
          <Heading size="7">{project?.name ?? "Project"}</Heading>
          <Text color="gray">{project?.id}</Text>
        </div>
        <Flex gap="2" wrap="wrap">
          <Button variant="soft" onClick={() => void refreshWorkspace(token, activeGroupId)}>
            Refresh
          </Button>
          <Button variant="soft" onClick={() => void navigate({ to: "/dashboard" })}>
            Dashboard
          </Button>
        </Flex>
      </Flex>

      <Card size="3">
        <Flex direction="column" gap="3">
          <Heading size="5">File Explorer</Heading>
          <Flex gap="2" wrap="wrap" align="center">
            <Text size="2" color="gray">
              Project group:
            </Text>
            <Badge variant="soft">{project?.group_id ?? "-"}</Badge>
            <Text size="2" color="gray">
              Token group:
            </Text>
            <Badge variant="soft">{activeGroupId || "-"}</Badge>
          </Flex>
          {!canInsertFolder ? (
            <Text size="2" color="amber">
              Folder insert operations are disabled because insert permissions force group_id
              to x-hasura-group-id.
            </Text>
          ) : null}
          <Separator size="4" />
          <FileExplorerTree
            parentFolder={currentParentFolder}
            breadcrumbFolders={breadcrumbFolders}
            currentFolders={currentFolders}
            currentImages={currentImages}
            disableMutations={isMutating}
            onCreateFolder={(parentId) => openCreateFolderDialog(parentId)}
            onUploadImage={(parentId) => openUploadImageDialog(parentId)}
            onOpenFolder={(folder: Folder) => setCurrentParentId(folder.id)}
            onGoRoot={() => setCurrentParentId(null)}
            onGoToFolder={(folder: Folder) => setCurrentParentId(folder.id)}
            onRenameFolder={(folder: Folder) =>
              (setRenameTargetNode({ id: folder.id, name: folder.name, kind: "folder" }),
              setRenameNodeName(folder.name))
            }
            onDeleteFolder={(folder: Folder) =>
              setDeleteTargetNode({ id: folder.id, name: folder.name, kind: "folder" })
            }
            onRenameImage={(image: Image) =>
              (setRenameTargetNode({ id: image.id, name: image.name, kind: "image" }),
              setRenameNodeName(image.name))
            }
            onDeleteImage={(image: Image) =>
              setDeleteTargetNode({ id: image.id, name: image.name, kind: "image" })
            }
          />
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
        open={isCreateFolderOpen}
        title="Create folder"
        description="Create a folder in the currently open location."
        label="Folder name"
        confirmLabel="Create"
        value={createFolderName}
        onValueChange={setCreateFolderName}
        pending={isMutating}
        onOpenChange={setIsCreateFolderOpen}
        onConfirm={confirmCreateFolder}
      />

      <UploadImageDialog
        open={isUploadImageOpen}
        name={uploadImageName}
        file={uploadImageFile}
        onNameChange={setUploadImageName}
        onFileChange={setUploadImageFile}
        pending={isMutating}
        onOpenChange={setIsUploadImageOpen}
        onConfirm={confirmUploadImage}
      />

      <TextInputDialog
        open={renameTargetNode !== null}
        title={`Rename ${renameTargetNode?.kind ?? "item"}`}
        description={
          renameTargetNode
            ? `Set a new name for "${renameTargetNode.name}".`
            : undefined
        }
        label="Name"
        confirmLabel="Rename"
        value={renameNodeName}
        onValueChange={setRenameNodeName}
        pending={isMutating}
        onOpenChange={(open) => {
          if (!open) {
            setRenameTargetNode(null);
          }
        }}
        onConfirm={confirmRenameNode}
      />

      <ConfirmDialog
        open={deleteTargetNode !== null}
        title={`Delete ${deleteTargetNode?.kind ?? "item"}`}
        description={
          deleteTargetNode
            ? `Delete "${deleteTargetNode.name}"?`
            : "Delete selected item?"
        }
        confirmLabel="Delete"
        pending={isMutating}
        danger
        onOpenChange={(open) => {
          if (!open) {
            setDeleteTargetNode(null);
          }
        }}
        onConfirm={confirmDeleteNode}
      />
    </>
  );
}
