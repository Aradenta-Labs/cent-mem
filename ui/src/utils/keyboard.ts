/**
 * Returns true if the keyboard event originated from an editable control
 * (input, textarea, select, or contenteditable element) where single-key
 * navigation/action shortcuts must be suppressed.
 */
export function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  const tag = target.tagName;
  if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return true;
  if (target.isContentEditable) return true;
  return Boolean(
    target.closest('[contenteditable="true"], [contenteditable=""], [contenteditable="plaintext-only"]')
  );
}

/**
 * Returns true if any blocking modal dialog is currently open in the DOM.
 * Pass `ignoreKinds` (matching `data-modal-kind`) to exclude specific overlays
 * such as the shortcuts modal itself when toggling `?` or the detail drawer
 * when checking for dialogs stacked above it.
 */
export function hasOpenModal(ignoreKinds: string[] = []): boolean {
  if (typeof document === 'undefined') return false;
  const dialogs = document.querySelectorAll('dialog[open], [role="dialog"][aria-modal="true"]');
  for (const el of dialogs) {
    const kind = el.getAttribute('data-modal-kind');
    if (kind && ignoreKinds.includes(kind)) continue;
    return true;
  }
  return false;
}

/**
 * Returns true if a non-modal popover/dropdown overlay (such as the Export menu
 * or Doctor Diagnostics popover) is currently open in the DOM.
 */
export function hasOpenPopover(): boolean {
  if (typeof document === 'undefined') return false;
  return document.querySelector('[data-popover-open="true"]') !== null;
}
