import * as vscode from 'vscode';
import { CentmemCLI } from './cli';
import { MemoryItem } from './types';

export class CentmemSidebarProvider implements vscode.WebviewViewProvider {
  public static readonly viewType = 'centmem.memoryView';
  private _view?: vscode.WebviewView;

  constructor(private readonly _extensionUri: vscode.Uri) {}

  public resolveWebviewView(
    webviewView: vscode.WebviewView,
    _context: vscode.WebviewViewResolveContext,
    _token: vscode.CancellationToken
  ) {
    this._view = webviewView;

    webviewView.webview.options = {
      enableScripts: true,
      localResourceRoots: [this._extensionUri],
    };

    webviewView.webview.html = this._getHtmlForWebview(webviewView.webview);

    webviewView.webview.onDidReceiveMessage(async (data) => {
      switch (data.type) {
        case 'recall': {
          await this.performRecall(data.query);
          break;
        }
        case 'saveSelection': {
          await vscode.commands.executeCommand('centmem.saveSelection');
          break;
        }
        case 'openUI': {
          await vscode.commands.executeCommand('centmem.openUI');
          break;
        }
        case 'copy': {
          if (data.text) {
            await vscode.env.clipboard.writeText(data.text);
            vscode.window.showInformationMessage('Copied to clipboard');
          }
          break;
        }
      }
    });
  }

  public async performRecall(query: string) {
    if (!this._view) {
      return;
    }

    const trimmed = query.trim();
    if (!trimmed) {
      this._view.webview.postMessage({ type: 'results', memories: [], query: '' });
      return;
    }

    this._view.webview.postMessage({ type: 'loading', loading: true });

    try {
      const resp = await CentmemCLI.recall(trimmed);
      if (this._view) {
        this._view.webview.postMessage({
          type: 'results',
          memories: resp.results || [],
          query: trimmed,
        });
      }
    } catch (err: any) {
      if (this._view) {
        this._view.webview.postMessage({
          type: 'error',
          message: err?.message || 'Failed to recall memory',
        });
      }
    } finally {
      if (this._view) {
        this._view.webview.postMessage({ type: 'loading', loading: false });
      }
    }
  }

  private _getHtmlForWebview(webview: vscode.Webview): string {
    const nonce = getNonce();

    return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src ${webview.cspSource} 'unsafe-inline'; script-src 'nonce-${nonce}';">
  <title>cent-mem</title>
  <style>
    body {
      padding: 12px;
      color: var(--vscode-foreground);
      font-family: var(--vscode-font-family);
      font-size: var(--vscode-font-size);
      background-color: var(--vscode-sideBar-background);
      margin: 0;
      box-sizing: border-box;
    }

    .header {
      margin-bottom: 12px;
    }

    .search-row {
      display: flex;
      gap: 6px;
      margin-bottom: 8px;
    }

    input[type="text"] {
      flex: 1;
      background: var(--vscode-input-background);
      color: var(--vscode-input-foreground);
      border: 1px solid var(--vscode-input-border, #3c3c3c);
      padding: 6px 8px;
      border-radius: 4px;
      outline: none;
      font-family: inherit;
      font-size: 12px;
    }

    input[type="text"]:focus {
      border-color: var(--vscode-focusBorder);
    }

    button {
      background: var(--vscode-button-background);
      color: var(--vscode-button-foreground);
      border: none;
      padding: 6px 10px;
      border-radius: 4px;
      cursor: pointer;
      font-size: 12px;
      font-weight: 500;
    }

    button:hover {
      background: var(--vscode-button-hoverBackground);
    }

    button.secondary {
      background: var(--vscode-button-secondaryBackground, #3a3d41);
      color: var(--vscode-button-secondaryForeground, #ffffff);
    }

    button.secondary:hover {
      background: var(--vscode-button-secondaryHoverBackground, #45494e);
    }

    .action-bar {
      display: flex;
      gap: 6px;
      margin-bottom: 12px;
    }

    .action-bar button {
      flex: 1;
      padding: 4px 6px;
      font-size: 11px;
    }

    .card-list {
      display: flex;
      flex-direction: column;
      gap: 10px;
    }

    .card {
      background: var(--vscode-editor-background);
      border: 1px solid var(--vscode-widget-border, #333);
      border-radius: 6px;
      padding: 10px;
      position: relative;
    }

    .card-meta {
      display: flex;
      align-items: center;
      gap: 6px;
      flex-wrap: wrap;
      margin-bottom: 6px;
      font-size: 11px;
    }

    .badge {
      background: var(--vscode-badge-background);
      color: var(--vscode-badge-foreground);
      padding: 2px 6px;
      border-radius: 10px;
      font-size: 10px;
      font-weight: 600;
      text-transform: uppercase;
    }

    .badge.type-note {
      background: #1e3a8a;
      color: #93c5fd;
    }

    .badge.type-fact {
      background: #14532d;
      color: #86efac;
    }

    .badge.type-log {
      background: #374151;
      color: #d1d5db;
    }

    .score {
      color: var(--vscode-descriptionForeground);
      font-size: 10px;
      margin-left: auto;
    }

    .card-content {
      font-size: 12px;
      line-height: 1.4;
      white-space: pre-wrap;
      word-break: break-word;
      margin-bottom: 8px;
    }

    .card-footer {
      display: flex;
      align-items: center;
      justify-content: space-between;
      border-top: 1px solid var(--vscode-widget-border, #2a2a2a);
      padding-top: 6px;
      font-size: 10px;
      color: var(--vscode-descriptionForeground);
    }

    .tags {
      display: flex;
      gap: 4px;
      flex-wrap: wrap;
    }

    .tag {
      background: var(--vscode-badge-background);
      padding: 1px 5px;
      border-radius: 3px;
      font-size: 9px;
    }

    .copy-btn {
      background: transparent;
      color: var(--vscode-descriptionForeground);
      padding: 2px 6px;
      font-size: 10px;
      border: 1px solid var(--vscode-widget-border, #444);
      border-radius: 3px;
    }

    .copy-btn:hover {
      color: var(--vscode-foreground);
      background: var(--vscode-toolbar-hoverBackground, #2a2a2a);
    }

    .empty-state {
      text-align: center;
      padding: 24px 12px;
      color: var(--vscode-descriptionForeground);
      font-size: 12px;
    }

    .error-banner {
      background: var(--vscode-inputValidation-errorBackground, #5a1d1d);
      border: 1px solid var(--vscode-inputValidation-errorBorder, #be1100);
      color: var(--vscode-foreground);
      padding: 8px;
      border-radius: 4px;
      margin-bottom: 10px;
      font-size: 11px;
    }

    .loading-spinner {
      text-align: center;
      padding: 16px;
      color: var(--vscode-descriptionForeground);
      font-size: 12px;
    }
  </style>
</head>
<body>
  <div class="header">
    <div class="search-row">
      <input type="text" id="searchInput" placeholder="Search memory or select code..." />
      <button id="searchBtn">Recall</button>
    </div>
    <div class="action-bar">
      <button id="saveBtn" class="secondary" title="Save active code selection">Save Selection</button>
      <button id="uiBtn" class="secondary" title="Open memory browser in browser">Open Web UI</button>
    </div>
  </div>

  <div id="errorArea" style="display: none;" class="error-banner"></div>
  <div id="loadingArea" style="display: none;" class="loading-spinner">Recalling memories...</div>
  <div id="cardList" class="card-list"></div>
  <div id="emptyArea" class="empty-state">
    Select code in the editor or type a query above to recall relevant memory.
  </div>

  <script nonce="${nonce}">
    const vscode = acquireVsCodeApi();
    const searchInput = document.getElementById('searchInput');
    const searchBtn = document.getElementById('searchBtn');
    const saveBtn = document.getElementById('saveBtn');
    const uiBtn = document.getElementById('uiBtn');
    const cardList = document.getElementById('cardList');
    const emptyArea = document.getElementById('emptyArea');
    const loadingArea = document.getElementById('loadingArea');
    const errorArea = document.getElementById('errorArea');

    searchBtn.addEventListener('click', () => {
      vscode.postMessage({ type: 'recall', query: searchInput.value });
    });

    searchInput.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') {
        vscode.postMessage({ type: 'recall', query: searchInput.value });
      }
    });

    saveBtn.addEventListener('click', () => {
      vscode.postMessage({ type: 'saveSelection' });
    });

    uiBtn.addEventListener('click', () => {
      vscode.postMessage({ type: 'openUI' });
    });

    window.addEventListener('message', (event) => {
      const message = event.data;
      switch (message.type) {
        case 'loading':
          loadingArea.style.display = message.loading ? 'block' : 'none';
          if (message.loading) {
            errorArea.style.display = 'none';
          }
          break;
        case 'error':
          errorArea.textContent = message.message;
          errorArea.style.display = 'block';
          loadingArea.style.display = 'none';
          break;
        case 'results':
          renderResults(message.memories || [], message.query || '');
          break;
      }
    });

    function renderResults(memories, query) {
      errorArea.style.display = 'none';
      loadingArea.style.display = 'none';
      cardList.innerHTML = '';

      if (query) {
        searchInput.value = query;
      }

      if (!memories || memories.length === 0) {
        emptyArea.style.display = 'block';
        emptyArea.textContent = query
          ? 'No memories found matching "' + query + '".'
          : 'Select code in the editor or type a query above to recall relevant memory.';
        return;
      }

      emptyArea.style.display = 'none';

      memories.forEach(mem => {
        const card = document.createElement('div');
        card.className = 'card';

        const meta = document.createElement('div');
        meta.className = 'card-meta';

        const typeBadge = document.createElement('span');
        typeBadge.className = 'badge type-' + (mem.type || 'note');
        typeBadge.textContent = mem.type || 'note';
        meta.appendChild(typeBadge);

        if (mem.scope) {
          const scopeBadge = document.createElement('span');
          scopeBadge.className = 'badge';
          scopeBadge.textContent = mem.scope;
          meta.appendChild(scopeBadge);
        }

        if (typeof mem.score === 'number') {
          const scoreEl = document.createElement('span');
          scoreEl.className = 'score';
          scoreEl.textContent = (mem.score * 100).toFixed(1) + '%';
          meta.appendChild(scoreEl);
        }

        card.appendChild(meta);

        const content = document.createElement('div');
        content.className = 'card-content';
        content.textContent = mem.content || '';
        card.appendChild(content);

        const footer = document.createElement('div');
        footer.className = 'card-footer';

        const tagsContainer = document.createElement('div');
        tagsContainer.className = 'tags';
        if (mem.tags && mem.tags.length > 0) {
          mem.tags.forEach(t => {
            const tag = document.createElement('span');
            tag.className = 'tag';
            tag.textContent = '#' + t;
            tagsContainer.appendChild(tag);
          });
        }
        footer.appendChild(tagsContainer);

        const copyBtn = document.createElement('button');
        copyBtn.className = 'copy-btn';
        copyBtn.textContent = 'Copy';
        copyBtn.addEventListener('click', () => {
          vscode.postMessage({ type: 'copy', text: mem.content });
        });
        footer.appendChild(copyBtn);

        card.appendChild(footer);
        cardList.appendChild(card);
      });
    }
  </script>
</body>
</html>`;
  }
}

function getNonce() {
  let text = '';
  const possible = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
  for (let i = 0; i < 32; i++) {
    text += possible.charAt(Math.floor(Math.random() * possible.length));
  }
  return text;
}
