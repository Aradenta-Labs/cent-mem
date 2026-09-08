# cent-mem for VS Code

Fast, local-first shared memory store for AI agents inside Visual Studio Code.

## Features

- **Contextual Memory Recall:** Automatically retrieves relevant decisions, conventions, notes, and facts as you navigate code and select text (debounced 300ms).
- **One-Click Memory Capture:** Right-click any selection and choose `Centmem: Save Selection as Memory` to store knowledge directly into centmem.
- **Dedicated Sidebar View:** Browse matching memories with relevance scores, type badges (`note`, `fact`, `log`), scope labels, and tags.
- **Embedded Web UI Integration:** Launch the embedded centmem Web UI Memory Browser directly from the extension title bar.

## Requirements

The `centmem` CLI binary must be installed and accessible either on your system `PATH` or configured via `centmem.binaryPath`.

## Extension Settings

This extension contributes the following settings:

* `centmem.binaryPath`: Path to the `centmem` CLI executable (default: `"centmem"`).
* `centmem.scope`: Default memory scope filter (e.g. `"project:my-app"`).
* `centmem.autoRecallOnSelection`: Automatically recall memories when text is selected (default: `true`).
* `centmem.top`: Number of recalled memories to display (default: `5`).
* `centmem.uiUrl`: Web UI URL to open with the "Open Web UI" action (default: `"http://127.0.0.1:4231"`).

## Usage

1. Open a workspace that uses `centmem`.
2. Click the `cent-mem` icon in the Activity Bar to open the Shared Memory sidebar.
3. Highlight any function or code block to see related memories appear instantly.
4. Type queries in the search box to perform manual hybrid semantic recall.
