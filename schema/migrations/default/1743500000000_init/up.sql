CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE public.groups (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT NOW()
);

CREATE TABLE public.users (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email text NOT NULL UNIQUE,
  display_name text NOT NULL,
  password_hash text NOT NULL,
  default_group_id uuid NOT NULL REFERENCES public.groups(id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL DEFAULT NOW(),
  last_login_at timestamptz
);

CREATE TABLE public.group_memberships (
  user_id uuid NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  group_id uuid NOT NULL REFERENCES public.groups(id) ON DELETE CASCADE,
  role text NOT NULL CHECK (role IN ('owner', 'member')),
  created_at timestamptz NOT NULL DEFAULT NOW(),
  PRIMARY KEY (user_id, group_id)
);

CREATE TABLE public.projects (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  group_id uuid NOT NULL REFERENCES public.groups(id) ON DELETE CASCADE,
  name text NOT NULL,
  created_by uuid NOT NULL REFERENCES public.users(id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL DEFAULT NOW()
);

CREATE TABLE public.nodes (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id uuid NOT NULL REFERENCES public.projects(id) ON DELETE CASCADE,
  parent_id uuid REFERENCES public.nodes(id) ON DELETE CASCADE,
  group_id uuid NOT NULL REFERENCES public.groups(id) ON DELETE RESTRICT,
  created_by uuid NOT NULL REFERENCES public.users(id) ON DELETE RESTRICT,
  node_type text NOT NULL CHECK (node_type IN ('folder', 'image')),
  name text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT NOW()
);

CREATE TABLE public.image_data (
  file_id uuid PRIMARY KEY REFERENCES public.nodes(id) ON DELETE CASCADE,
  mime_type text NOT NULL,
  size_bytes bigint NOT NULL,
  storage_key text NOT NULL,
  thumbnail_key text,
  created_at timestamptz NOT NULL DEFAULT NOW()
);

CREATE TABLE public.node_parents_children (
  parent_id uuid NOT NULL REFERENCES public.nodes(id) ON DELETE CASCADE,
  child_id uuid NOT NULL REFERENCES public.nodes(id) ON DELETE CASCADE,
  depth integer NOT NULL CHECK (depth >= 0),
  PRIMARY KEY (parent_id, child_id)
);

CREATE INDEX idx_group_memberships_group_id ON public.group_memberships(group_id);
CREATE INDEX idx_projects_group_id ON public.projects(group_id);
CREATE INDEX idx_nodes_project_id ON public.nodes(project_id);
CREATE INDEX idx_nodes_parent_id ON public.nodes(parent_id);
CREATE INDEX idx_nodes_group_id ON public.nodes(group_id);
CREATE INDEX idx_nodes_node_type ON public.nodes(node_type);
CREATE INDEX idx_node_parents_children_child_id ON public.node_parents_children(child_id);

CREATE OR REPLACE FUNCTION public.validate_node_parent_consistency()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  parent_project_id uuid;
  parent_group_id uuid;
  parent_type text;
  project_group_id uuid;
BEGIN
  SELECT p.group_id
  INTO project_group_id
  FROM public.projects p
  WHERE p.id = NEW.project_id;

  IF project_group_id IS NULL THEN
    RAISE EXCEPTION 'project % not found', NEW.project_id;
  END IF;

  IF project_group_id <> NEW.group_id THEN
    RAISE EXCEPTION 'node.group_id must equal project.group_id';
  END IF;

  IF NEW.parent_id IS NULL THEN
    RETURN NEW;
  END IF;

  SELECT n.project_id, n.group_id, n.node_type
  INTO parent_project_id, parent_group_id, parent_type
  FROM public.nodes n
  WHERE n.id = NEW.parent_id;

  IF parent_project_id IS NULL THEN
    RAISE EXCEPTION 'parent node % not found', NEW.parent_id;
  END IF;

  IF parent_type <> 'folder' THEN
    RAISE EXCEPTION 'parent node must be a folder';
  END IF;

  IF parent_project_id <> NEW.project_id THEN
    RAISE EXCEPTION 'node.project_id must equal parent.project_id';
  END IF;

  IF parent_group_id <> NEW.group_id THEN
    RAISE EXCEPTION 'node.group_id must equal parent.group_id';
  END IF;

  RETURN NEW;
END;
$$;

CREATE TRIGGER trg_validate_node_parent_consistency
BEFORE INSERT OR UPDATE OF parent_id, project_id, group_id, node_type
ON public.nodes
FOR EACH ROW
EXECUTE FUNCTION public.validate_node_parent_consistency();

CREATE OR REPLACE FUNCTION public.validate_image_data_node_type()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  ntype text;
BEGIN
  SELECT node_type
  INTO ntype
  FROM public.nodes
  WHERE id = NEW.file_id;

  IF ntype IS NULL THEN
    RAISE EXCEPTION 'node % not found for image_data', NEW.file_id;
  END IF;

  IF ntype <> 'image' THEN
    RAISE EXCEPTION 'image_data.file_id must reference an image node';
  END IF;

  RETURN NEW;
END;
$$;

CREATE TRIGGER trg_validate_image_data_node_type
BEFORE INSERT OR UPDATE OF file_id
ON public.image_data
FOR EACH ROW
EXECUTE FUNCTION public.validate_image_data_node_type();

CREATE OR REPLACE FUNCTION public.rebuild_node_parents_children()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  DELETE FROM public.node_parents_children;

  WITH RECURSIVE closure AS (
    SELECT
      n.id AS child_id,
      n.id AS parent_id,
      0 AS depth
    FROM public.nodes n
    UNION ALL
    SELECT
      c.child_id,
      p.parent_id,
      c.depth + 1
    FROM closure c
    JOIN public.nodes p ON p.id = c.parent_id
    WHERE p.parent_id IS NOT NULL
  )
  INSERT INTO public.node_parents_children (parent_id, child_id, depth)
  SELECT
    parent_id,
    child_id,
    MIN(depth) AS depth
  FROM closure
  GROUP BY parent_id, child_id;

  RETURN NULL;
END;
$$;

CREATE TRIGGER trg_rebuild_node_parents_children
AFTER INSERT OR UPDATE OR DELETE
ON public.nodes
FOR EACH STATEMENT
EXECUTE FUNCTION public.rebuild_node_parents_children();

CREATE OR REPLACE VIEW public.folders AS
SELECT
  n.id,
  n.project_id,
  n.parent_id,
  n.group_id,
  n.created_by,
  n.name,
  n.created_at
FROM public.nodes n
WHERE n.node_type = 'folder';

CREATE OR REPLACE VIEW public.images AS
SELECT
  n.id,
  n.project_id,
  n.parent_id,
  n.group_id,
  n.created_by,
  n.name,
  i.mime_type,
  i.size_bytes,
  i.storage_key,
  i.thumbnail_key,
  n.created_at
FROM public.nodes n
JOIN public.image_data i ON i.file_id = n.id
WHERE n.node_type = 'image';
