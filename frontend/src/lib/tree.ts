export const ROOT_PARENT = "__root__";

type NamedTreeNode = {
  parent_id: string | null;
  name: string;
};

export function byName<T extends { name: string }>(a: T, b: T): number {
  return a.name.localeCompare(b.name, undefined, { sensitivity: "base" });
}

export function toParentMap<T extends NamedTreeNode>(rows: T[]): Map<string, T[]> {
  const map = new Map<string, T[]>();
  for (const row of rows) {
    const key = row.parent_id ?? ROOT_PARENT;
    if (!map.has(key)) {
      map.set(key, []);
    }
    map.get(key)?.push(row);
  }
  for (const items of map.values()) {
    items.sort(byName);
  }
  return map;
}

export function pickImageFile(): Promise<File | null> {
  return new Promise((resolve) => {
    const input = document.createElement("input");
    input.type = "file";
    input.accept = "image/*";
    input.onchange = () => resolve(input.files?.[0] ?? null);
    input.click();
  });
}
