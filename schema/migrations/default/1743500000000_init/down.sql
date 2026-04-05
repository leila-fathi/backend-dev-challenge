DROP VIEW IF EXISTS public.images;
DROP VIEW IF EXISTS public.folders;

DROP TRIGGER IF EXISTS trg_rebuild_node_parents_children ON public.nodes;
DROP FUNCTION IF EXISTS public.rebuild_node_parents_children();

DROP TRIGGER IF EXISTS trg_validate_image_data_node_type ON public.image_data;
DROP FUNCTION IF EXISTS public.validate_image_data_node_type();

DROP TRIGGER IF EXISTS trg_validate_node_parent_consistency ON public.nodes;
DROP FUNCTION IF EXISTS public.validate_node_parent_consistency();

DROP TABLE IF EXISTS public.node_parents_children;
DROP TABLE IF EXISTS public.image_data;
DROP TABLE IF EXISTS public.nodes;
DROP TABLE IF EXISTS public.projects;
DROP TABLE IF EXISTS public.group_memberships;
DROP TABLE IF EXISTS public.users;
DROP TABLE IF EXISTS public.groups;
