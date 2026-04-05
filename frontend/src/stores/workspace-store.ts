import { create } from "zustand";
import { fetchWorkspace } from "../api";
import type { WorkspaceData } from "../api";

const emptyWorkspace: WorkspaceData = {
  groups: [],
  projects: [],
  folders: [],
  images: [],
};

export type WorkspaceState = {
  workspace: WorkspaceData;
  selectedGroupId: string;
  isLoading: boolean;
  error: string;
  status: string;
  clearWorkspace: () => void;
  clearError: () => void;
  setStatus: (status: string) => void;
  setSelectedGroupId: (groupId: string) => void;
  refreshWorkspace: (token: string, preferredGroupId?: string) => Promise<void>;
};

export const useWorkspaceStore = create<WorkspaceState>((set, get) => ({
  workspace: emptyWorkspace,
  selectedGroupId: "",
  isLoading: false,
  error: "",
  status: "Not logged in.",
  clearWorkspace: () => {
    set({
      workspace: emptyWorkspace,
      selectedGroupId: "",
      isLoading: false,
      error: "",
      status: "Not logged in.",
    });
  },
  clearError: () => set({ error: "" }),
  setStatus: (status) => set({ status }),
  setSelectedGroupId: (groupId) =>
    set((state) => {
      if (state.selectedGroupId === groupId) {
        return state;
      }
      return { selectedGroupId: groupId };
    }),
  refreshWorkspace: async (token, preferredGroupId) => {
    if (!token) {
      return;
    }

    set({ isLoading: true, error: "" });
    try {
      const data = await fetchWorkspace(token);
      const groupIds = data.groups.map((group) => group.id);
      const currentGroupId = get().selectedGroupId;
      let nextGroupId = "";

      if (currentGroupId && groupIds.includes(currentGroupId)) {
        nextGroupId = currentGroupId;
      } else if (preferredGroupId && groupIds.includes(preferredGroupId)) {
        nextGroupId = preferredGroupId;
      } else {
        nextGroupId = groupIds[0] ?? "";
      }

      set((state) => ({
        workspace: data,
        selectedGroupId:
          state.selectedGroupId === nextGroupId ? state.selectedGroupId : nextGroupId,
        isLoading: false,
      }));
    } catch (error) {
      const message = error instanceof Error ? error.message : "Failed to load workspace";
      set({ isLoading: false, error: message });
    }
  },
}));
