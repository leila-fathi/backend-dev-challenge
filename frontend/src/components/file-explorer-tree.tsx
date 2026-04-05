import {
  ArchiveIcon,
  ChevronRightIcon,
  HomeIcon,
  ImageIcon,
} from "@radix-ui/react-icons";
import { Button, Card, Flex, Heading, Text } from "@radix-ui/themes";
import type { Folder, Image } from "../api";

type FileExplorerTreeProps = {
  parentFolder: Folder | null;
  breadcrumbFolders: Folder[];
  currentFolders: Folder[];
  currentImages: Image[];
  disableMutations: boolean;
  onCreateFolder: (parentId: string | null) => void;
  onUploadImage: (parentId: string | null) => void;
  onOpenFolder: (folder: Folder) => void;
  onGoRoot: () => void;
  onGoToFolder: (folder: Folder) => void;
  onRenameFolder: (folder: Folder) => void;
  onDeleteFolder: (folder: Folder) => void;
  onRenameImage: (image: Image) => void;
  onDeleteImage: (image: Image) => void;
};

export function FileExplorerTree({
  parentFolder,
  breadcrumbFolders,
  currentFolders,
  currentImages,
  disableMutations,
  onCreateFolder,
  onUploadImage,
  onOpenFolder,
  onGoRoot,
  onGoToFolder,
  onRenameFolder,
  onDeleteFolder,
  onRenameImage,
  onDeleteImage,
}: FileExplorerTreeProps) {
  const currentParentId = parentFolder?.id ?? null;

  return (
    <Flex direction="column" gap="3">
      <Flex gap="2" align="center" wrap="wrap">
        <Button variant="soft" size="1" onClick={onGoRoot}>
          <HomeIcon /> Root
        </Button>
        {breadcrumbFolders.map((folder) => (
          <Flex key={folder.id} align="center" gap="1">
            <ChevronRightIcon />
            <Button size="1" variant="ghost" onClick={() => onGoToFolder(folder)}>
              {folder.name}
            </Button>
          </Flex>
        ))}
      </Flex>

      <Flex justify="between" align="center" wrap="wrap" gap="2">
        <Heading size="4">
          {parentFolder ? `Folder: ${parentFolder.name}` : "Root"}
        </Heading>
        <Text size="2" color="gray">
          Showing direct children only
        </Text>
      </Flex>

      <Flex gap="2" wrap="wrap">
        <Button disabled={disableMutations} onClick={() => onCreateFolder(currentParentId)}>
          New Folder
        </Button>
        <Button variant="soft" onClick={() => onUploadImage(currentParentId)}>
          Upload Image
        </Button>
      </Flex>

      <div className="explorer-grid">
        {currentFolders.map((folder) => (
          <Card key={folder.id} className="entry-card">
            <Flex direction="column" gap="3">
              <Flex align="center" gap="2">
                <ArchiveIcon className="entry-icon" />
                <Text weight="medium" className="entry-name">
                  {folder.name}
                </Text>
              </Flex>
              <Flex gap="2" wrap="wrap">
                <Button size="1" onClick={() => onOpenFolder(folder)}>
                  Open
                </Button>
                <Button
                  size="1"
                  variant="soft"
                  disabled={disableMutations}
                  onClick={() => onRenameFolder(folder)}
                >
                  Rename
                </Button>
                <Button
                  size="1"
                  color="red"
                  variant="outline"
                  disabled={disableMutations}
                  onClick={() => onDeleteFolder(folder)}
                >
                  Delete
                </Button>
              </Flex>
            </Flex>
          </Card>
        ))}

        {currentImages.map((image) => {
          const thumbnailUrl = image.thumbnail_key;
          return (
            <Card key={image.id} className="entry-card">
              <Flex direction="column" gap="3">
                <div className="thumb-wrapper">
                  {thumbnailUrl ? (
                    <img
                      src={thumbnailUrl}
                      alt={image.name}
                      className="thumb-image"
                      loading="lazy"
                    />
                  ) : (
                    <div className="thumb-placeholder">
                      <ImageIcon className="entry-icon" />
                    </div>
                  )}
                </div>
                <Text weight="medium" className="entry-name">
                  {image.name}
                </Text>
                <Text size="1" color="gray">
                  {image.mime_type} - {image.size_bytes} bytes
                </Text>
                <Flex gap="2" wrap="wrap">
                  <Button
                    size="1"
                    variant="soft"
                    disabled={disableMutations}
                    onClick={() => onRenameImage(image)}
                  >
                    Rename
                  </Button>
                  <Button
                    size="1"
                    color="red"
                    variant="outline"
                    disabled={disableMutations}
                    onClick={() => onDeleteImage(image)}
                  >
                    Delete
                  </Button>
                </Flex>
              </Flex>
            </Card>
          );
        })}

        {currentFolders.length === 0 && currentImages.length === 0 ? (
          <Text size="2" color="gray">
            No folders or images in this folder.
          </Text>
        ) : null}
      </div>
    </Flex>
  );
}
