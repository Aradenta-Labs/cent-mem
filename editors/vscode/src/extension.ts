import * as path from 'path';
import * as vscode from 'vscode';
import { CentmemCLI } from './cli';
import { CentmemSidebarProvider } from './sidebar';

export function activate(context: vscode.ExtensionContext) {
  const sidebarProvider = new CentmemSidebarProvider(context.extensionUri);

  context.subscriptions.push(
    vscode.window.registerWebviewViewProvider(
      CentmemSidebarProvider.viewType,
      sidebarProvider
    )
  );

  // Command: centmem.recall
  context.subscriptions.push(
    vscode.commands.registerCommand('centmem.recall', async () => {
      const editor = vscode.window.activeTextEditor;
      let defaultQuery = '';
      if (editor && !editor.selection.isEmpty) {
        defaultQuery = editor.document.getText(editor.selection).trim();
      }

      const query = await vscode.window.showInputBox({
        prompt: 'Search cent-mem shared memory',
        placeHolder: 'e.g. SQLite vector index architecture, API conventions',
        value: defaultQuery,
      });

      if (query && query.trim()) {
        await sidebarProvider.performRecall(query.trim());
      }
    })
  );

  // Command: centmem.saveSelection
  context.subscriptions.push(
    vscode.commands.registerCommand('centmem.saveSelection', async () => {
      const editor = vscode.window.activeTextEditor;
      if (!editor || editor.selection.isEmpty) {
        vscode.window.showWarningMessage('Please select text in the editor to save as memory.');
        return;
      }

      const selectedText = editor.document.getText(editor.selection).trim();
      if (!selectedText) {
        return;
      }

      const fileName = path.basename(editor.document.fileName);
      const content = `[${fileName}] ${selectedText}`;

      try {
        const result = await CentmemCLI.put(content, { tags: ['code', 'vscode'] });
        if (result.ok && result.id) {
          vscode.window.showInformationMessage(`Saved memory #${result.id} to cent-mem.`);
        } else {
          vscode.window.showInformationMessage('Memory saved successfully.');
        }
      } catch (err: any) {
        vscode.window.showErrorMessage(`Failed to save memory: ${err?.message || err}`);
      }
    })
  );

  // Command: centmem.refreshSidebar
  context.subscriptions.push(
    vscode.commands.registerCommand('centmem.refreshSidebar', async () => {
      const query = getContextualQuery();
      if (query) {
        await sidebarProvider.performRecall(query);
      }
    })
  );

  // Command: centmem.openUI
  context.subscriptions.push(
    vscode.commands.registerCommand('centmem.openUI', async () => {
      const config = vscode.workspace.getConfiguration('centmem');
      const uiUrl = config.get<string>('uiUrl', 'http://127.0.0.1:4231');
      try {
        await vscode.env.openExternal(vscode.Uri.parse(uiUrl));
      } catch (err: any) {
        vscode.window.showErrorMessage(`Failed to open UI at ${uiUrl}: ${err?.message || err}`);
      }
    })
  );

  // Debounced auto-recall on selection change (300ms)
  let debounceTimer: NodeJS.Timeout | undefined;

  const triggerAutoRecall = () => {
    const config = vscode.workspace.getConfiguration('centmem');
    const enabled = config.get<boolean>('autoRecallOnSelection', true);
    if (!enabled) {
      return;
    }

    if (debounceTimer) {
      clearTimeout(debounceTimer);
    }

    debounceTimer = setTimeout(async () => {
      const query = getContextualQuery();
      if (query) {
        await sidebarProvider.performRecall(query);
      }
    }, 300);
  };

  context.subscriptions.push(
    vscode.window.onDidChangeTextEditorSelection(() => triggerAutoRecall())
  );

  context.subscriptions.push(
    vscode.window.onDidChangeActiveTextEditor(() => triggerAutoRecall())
  );
}

function getContextualQuery(): string {
  const editor = vscode.window.activeTextEditor;
  if (!editor) {
    return '';
  }

  const fileName = path.basename(editor.document.fileName);
  if (!editor.selection.isEmpty) {
    const selected = editor.document.getText(editor.selection).trim();
    if (selected.length > 0) {
      // Truncate very long multi-line selections for search efficiency
      const snippet = selected.length > 200 ? selected.slice(0, 200) : selected;
      return `${fileName} ${snippet}`.trim();
    }
  }

  // Fall back to current line or symbol context
  const currentLine = editor.document.lineAt(editor.selection.active.line).text.trim();
  if (currentLine.length > 3) {
    const snippet = currentLine.length > 100 ? currentLine.slice(0, 100) : currentLine;
    return `${fileName} ${snippet}`.trim();
  }

  return fileName;
}

export function deactivate() {}
