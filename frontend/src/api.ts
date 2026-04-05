const BACKEND_GRAPHQL_URL =
  import.meta.env.VITE_BACKEND_GRAPHQL_URL ?? "http://localhost:8081/graphql";
const HASURA_GRAPHQL_URL =
  import.meta.env.VITE_HASURA_GRAPHQL_URL ?? "http://localhost:8080/v1/graphql";
const UPLOAD_IMAGE_URL =
  import.meta.env.VITE_UPLOAD_IMAGE_URL ?? "http://localhost:8081/upload/image";

type GraphQLError = {
  message?: string;
};

type GraphQLPayload<TData> = {
  data?: TData;
  errors?: GraphQLError[];
};

type GraphqlRequestArgs = {
  url: string;
  query: string;
  token?: string;
  variables?: Record<string, unknown>;
};

export type AuthPayload = {
  token: string;
  tokenExpiresAt: number;
  userId: string;
  groupId: string;
  email: string;
  displayName: string;
};

export type Group = {
  id: string;
  name: string;
  created_at: string;
};

export type Project = {
  id: string;
  group_id: string;
  name: string;
  created_by: string;
  created_at: string;
};

export type Folder = {
  id: string;
  project_id: string;
  parent_id: string | null;
  group_id: string;
  created_by: string;
  name: string;
  created_at: string;
};

export type Image = {
  id: string;
  project_id: string;
  parent_id: string | null;
  group_id: string;
  created_by: string;
  name: string;
  mime_type: string;
  size_bytes: number;
  storage_key: string;
  thumbnail_key: string | null;
  created_at: string;
};

export type WorkspaceData = {
  groups: Group[];
  projects: Project[];
  folders: Folder[];
  images: Image[];
};

export type UploadImageResponse = {
  id: string;
  projectId: string;
  parentId: string | null;
  name: string;
  mimeType: string;
  sizeBytes: number;
  storageKey: string;
  thumbnailKey: string | null;
  createdAt: string;
};

type NodeRecord = {
  id: string;
  project_id: string;
  parent_id: string | null;
  group_id: string;
  node_type: "folder" | "image";
  name: string;
  created_by: string;
  created_at: string;
};

async function graphqlRequest<TData>({
  url,
  token,
  query,
  variables = {},
}: GraphqlRequestArgs): Promise<TData> {
  const response = await fetch(url, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: JSON.stringify({ query, variables }),
  });

  const payload = (await response.json().catch(() => null)) as GraphQLPayload<TData> | null;
  if (!response.ok || !payload || payload.errors || payload.data === undefined) {
    const message =
      payload?.errors?.[0]?.message ?? `Request failed with status ${response.status}`;
    throw new Error(message);
  }

  return payload.data;
}

export async function login(email: string, password: string): Promise<AuthPayload> {
  type LoginResponse = {
    login: AuthPayload;
  };

  const data = await graphqlRequest<LoginResponse>({
    url: BACKEND_GRAPHQL_URL,
    query: `
      mutation Login($input: LoginInput!) {
        login(input: $input) {
          token
          tokenExpiresAt
          userId
          groupId
          email
          displayName
        }
      }
    `,
    variables: { input: { email, password } },
  });
  return data.login;
}

export async function renewToken(token: string): Promise<AuthPayload> {
  type RenewTokenResponse = {
    renewToken: AuthPayload;
  };

  const data = await graphqlRequest<RenewTokenResponse>({
    url: BACKEND_GRAPHQL_URL,
    token,
    query: `
      mutation RenewToken {
        renewToken {
          token
          tokenExpiresAt
          userId
          groupId
          email
          displayName
        }
      }
    `,
  });
  return data.renewToken;
}

export async function switchGroup(token: string, groupId: string): Promise<AuthPayload> {
  type SwitchGroupResponse = {
    switchGroup: AuthPayload;
  };

  const data = await graphqlRequest<SwitchGroupResponse>({
    url: BACKEND_GRAPHQL_URL,
    token,
    query: `
      mutation SwitchGroup($groupId: ID!) {
        switchGroup(groupId: $groupId) {
          token
          tokenExpiresAt
          userId
          groupId
          email
          displayName
        }
      }
    `,
    variables: { groupId },
  });
  return data.switchGroup;
}

export async function fetchWorkspace(token: string): Promise<WorkspaceData> {
  return graphqlRequest<WorkspaceData>({
    url: HASURA_GRAPHQL_URL,
    token,
    query: `
      query FrontendWorkspace {
        groups(order_by: { created_at: asc }) {
          id
          name
          created_at
        }
        projects(order_by: { created_at: asc }) {
          id
          group_id
          name
          created_by
          created_at
        }
        folders(order_by: { created_at: asc }) {
          id
          project_id
          parent_id
          group_id
          created_by
          name
          created_at
        }
        images(order_by: { created_at: asc }) {
          id
          project_id
          parent_id
          group_id
          created_by
          name
          mime_type
          size_bytes
          storage_key
          thumbnail_key
          created_at
        }
      }
    `,
  });
}

export async function createProject(
  token: string,
  { name }: { name: string },
): Promise<Project> {
  type CreateProjectResponse = {
    insert_projects_one: Project;
  };

  const data = await graphqlRequest<CreateProjectResponse>({
    url: HASURA_GRAPHQL_URL,
    token,
    query: `
      mutation CreateProject($name: String!) {
        insert_projects_one(object: { name: $name }) {
          id
          group_id
          name
          created_by
          created_at
        }
      }
    `,
    variables: { name },
  });
  return data.insert_projects_one;
}

export async function renameProject(
  token: string,
  { id, name }: { id: string; name: string },
): Promise<Project | null> {
  type RenameProjectResponse = {
    update_projects_by_pk: Project | null;
  };

  const data = await graphqlRequest<RenameProjectResponse>({
    url: HASURA_GRAPHQL_URL,
    token,
    query: `
      mutation RenameProject($id: uuid!, $name: String!) {
        update_projects_by_pk(pk_columns: { id: $id }, _set: { name: $name }) {
          id
          group_id
          name
          created_by
          created_at
        }
      }
    `,
    variables: { id, name },
  });
  return data.update_projects_by_pk;
}

export async function deleteProject(
  token: string,
  { id }: { id: string },
): Promise<{ id: string } | null> {
  type DeleteProjectResponse = {
    delete_projects_by_pk: { id: string } | null;
  };

  const data = await graphqlRequest<DeleteProjectResponse>({
    url: HASURA_GRAPHQL_URL,
    token,
    query: `
      mutation DeleteProject($id: uuid!) {
        delete_projects_by_pk(id: $id) {
          id
        }
      }
    `,
    variables: { id },
  });
  return data.delete_projects_by_pk;
}

export async function createFolder(
  token: string,
  {
    projectId,
    parentId,
    name,
  }: {
    projectId: string;
    parentId: string | null;
    name: string;
  },
): Promise<NodeRecord> {
  type CreateFolderResponse = {
    insert_nodes_one: NodeRecord;
  };

  const data = await graphqlRequest<CreateFolderResponse>({
    url: HASURA_GRAPHQL_URL,
    token,
    query: `
      mutation CreateFolder(
        $projectId: uuid!,
        $parentId: uuid,
        $name: String!
      ) {
        insert_nodes_one(
          object: {
            project_id: $projectId,
            parent_id: $parentId,
            node_type: "folder",
            name: $name
          }
        ) {
          id
          project_id
          parent_id
          group_id
          node_type
          name
          created_by
          created_at
        }
      }
    `,
    variables: { projectId, parentId, name },
  });
  return data.insert_nodes_one;
}

export async function renameNode(
  token: string,
  { id, name }: { id: string; name: string },
): Promise<NodeRecord | null> {
  type RenameNodeResponse = {
    update_nodes_by_pk: NodeRecord | null;
  };

  const data = await graphqlRequest<RenameNodeResponse>({
    url: HASURA_GRAPHQL_URL,
    token,
    query: `
      mutation RenameNode($id: uuid!, $name: String!) {
        update_nodes_by_pk(pk_columns: { id: $id }, _set: { name: $name }) {
          id
          project_id
          parent_id
          group_id
          node_type
          name
          created_by
          created_at
        }
      }
    `,
    variables: { id, name },
  });
  return data.update_nodes_by_pk;
}

export async function deleteNode(
  token: string,
  { id }: { id: string },
): Promise<{ id: string } | null> {
  type DeleteNodeResponse = {
    delete_nodes_by_pk: { id: string } | null;
  };

  const data = await graphqlRequest<DeleteNodeResponse>({
    url: HASURA_GRAPHQL_URL,
    token,
    query: `
      mutation DeleteNode($id: uuid!) {
        delete_nodes_by_pk(id: $id) {
          id
        }
      }
    `,
    variables: { id },
  });
  return data.delete_nodes_by_pk;
}

export async function uploadImage(
  token: string,
  {
    projectId,
    parentId,
    name,
    file,
  }: {
    projectId: string;
    parentId: string | null;
    name: string;
    file: File;
  },
): Promise<UploadImageResponse> {
  const body = new FormData();
  body.append("projectId", projectId);
  if (parentId) {
    body.append("parentId", parentId);
  }
  if (name) {
    body.append("name", name);
  }
  body.append("image", file);

  const response = await fetch(UPLOAD_IMAGE_URL, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${token}`,
    },
    body,
  });

  const payload = (await response.json().catch(() => null)) as UploadImageResponse | null;
  if (!response.ok || !payload) {
    throw new Error(`Upload failed with status ${response.status}`);
  }

  return payload;
}

export const runtimeConfig = {
  backendGraphqlUrl: BACKEND_GRAPHQL_URL,
  hasuraGraphqlUrl: HASURA_GRAPHQL_URL,
  uploadImageUrl: UPLOAD_IMAGE_URL,
};
