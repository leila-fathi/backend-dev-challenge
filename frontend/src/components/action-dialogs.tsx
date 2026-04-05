import type { ChangeEvent } from "react";
import { Button, Dialog, Flex, Text, TextField } from "@radix-ui/themes";

type TextInputDialogProps = {
  open: boolean;
  title: string;
  description?: string;
  label: string;
  confirmLabel: string;
  value: string;
  onValueChange: (value: string) => void;
  pending?: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: (value: string) => Promise<boolean>;
};

export function TextInputDialog({
  open,
  title,
  description,
  label,
  confirmLabel,
  value,
  onValueChange,
  pending = false,
  onOpenChange,
  onConfirm,
}: TextInputDialogProps) {
  const safeValue = typeof value === "string" ? value : "";

  async function handleConfirm() {
    const nextValue = safeValue.trim();
    if (!nextValue) {
      return;
    }
    const success = await onConfirm(nextValue);
    if (success) {
      onOpenChange(false);
    }
  }

  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Content size="2">
        <Dialog.Title>{title}</Dialog.Title>
        {description ? <Dialog.Description>{description}</Dialog.Description> : null}
        <Flex direction="column" gap="2" mt="3">
          <Text size="2">{label}</Text>
          <TextField.Root
            value={safeValue}
            onChange={(event) => onValueChange(event.target.value)}
            placeholder={label}
            disabled={pending}
          />
        </Flex>
        <Flex justify="end" gap="2" mt="4">
          <Button variant="soft" color="gray" onClick={() => onOpenChange(false)} disabled={pending}>
            Cancel
          </Button>
          <Button onClick={() => void handleConfirm()} disabled={pending || !safeValue.trim()}>
            {confirmLabel}
          </Button>
        </Flex>
      </Dialog.Content>
    </Dialog.Root>
  );
}

type ConfirmDialogProps = {
  open: boolean;
  title: string;
  description: string;
  confirmLabel: string;
  pending?: boolean;
  danger?: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => Promise<boolean>;
};

export function ConfirmDialog({
  open,
  title,
  description,
  confirmLabel,
  pending = false,
  danger = false,
  onOpenChange,
  onConfirm,
}: ConfirmDialogProps) {
  async function handleConfirm() {
    const success = await onConfirm();
    if (success) {
      onOpenChange(false);
    }
  }

  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Content size="2">
        <Dialog.Title>{title}</Dialog.Title>
        <Dialog.Description>{description}</Dialog.Description>
        <Flex justify="end" gap="2" mt="4">
          <Button variant="soft" color="gray" onClick={() => onOpenChange(false)} disabled={pending}>
            Cancel
          </Button>
          <Button
            color={danger ? "red" : "indigo"}
            onClick={() => void handleConfirm()}
            disabled={pending}
          >
            {confirmLabel}
          </Button>
        </Flex>
      </Dialog.Content>
    </Dialog.Root>
  );
}

type UploadImageDialogProps = {
  open: boolean;
  name: string;
  file: File | null;
  onNameChange: (value: string) => void;
  onFileChange: (file: File | null) => void;
  pending?: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: (payload: { name: string; file: File }) => Promise<boolean>;
};

export function UploadImageDialog({
  open,
  name,
  file,
  onNameChange,
  onFileChange,
  pending = false,
  onOpenChange,
  onConfirm,
}: UploadImageDialogProps) {
  const safeName = typeof name === "string" ? name : "";

  function handleFileChange(event: ChangeEvent<HTMLInputElement>) {
    onFileChange(event.target.files?.[0] ?? null);
  }

  async function handleConfirm() {
    if (!file || !safeName.trim()) {
      return;
    }
    const success = await onConfirm({ name: safeName.trim(), file });
    if (success) {
      onOpenChange(false);
    }
  }

  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Content size="2">
        <Dialog.Title>Upload image</Dialog.Title>
        <Dialog.Description>Select a file and set image name.</Dialog.Description>
        <Flex direction="column" gap="3" mt="3">
          <div className="modal-field">
            <Text size="2">Image file</Text>
            <input className="file-input" type="file" accept="image/*" onChange={handleFileChange} />
          </div>
          <div className="modal-field">
            <Text size="2">Image name</Text>
            <TextField.Root
              value={safeName}
              onChange={(event) => onNameChange(event.target.value)}
              placeholder="image name"
              disabled={pending}
            />
          </div>
        </Flex>
        <Flex justify="end" gap="2" mt="4">
          <Button variant="soft" color="gray" onClick={() => onOpenChange(false)} disabled={pending}>
            Cancel
          </Button>
          <Button onClick={() => void handleConfirm()} disabled={pending || !file || !safeName.trim()}>
            Upload
          </Button>
        </Flex>
      </Dialog.Content>
    </Dialog.Root>
  );
}
