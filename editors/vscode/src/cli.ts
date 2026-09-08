import * as cp from 'child_process';
import type * as vscodeTypes from 'vscode';
import { MemoryItem, PutResult, RecallResult, StatsResult } from './types';

function getVSCode(): typeof vscodeTypes | undefined {
  try {
    return require('vscode');
  } catch {
    return undefined;
  }
}

export function buildRecallArgs(query: string, options?: { scope?: string; top?: number; type?: string; tags?: string[] }): string[] {
  const args = ['recall', query];
  if (options?.scope && options.scope.trim() !== '') {
    args.push('--scope', options.scope.trim());
  }
  if (options?.top) {
    args.push('--top', options.top.toString());
  }
  if (options?.type) {
    args.push('--type', options.type);
  }
  if (options?.tags && options.tags.length > 0) {
    args.push('--tags', options.tags.join(','));
  }
  return args;
}

export function buildPutArgs(content: string, options?: { scope?: string; type?: string; tags?: string[] }): string[] {
  const args = ['put', '--content', content];
  const scope = options?.scope || 'global';
  args.push('--scope', scope);
  args.push('--type', options?.type || 'note');
  args.push('--tags', (options?.tags && options.tags.length > 0) ? options.tags.join(',') : 'code');
  return args;
}

export function parseCLIError(stderr: string, fallbackMessage: string): Error {
  try {
    const parsed = JSON.parse(stderr);
    if (parsed && parsed.error && parsed.error.message) {
      return new Error(parsed.error.message);
    }
  } catch {}
  return new Error(stderr.trim() || fallbackMessage);
}

export class CentmemCLI {
  public static getBinaryPath(): string {
    const vscode = getVSCode();
    if (vscode && vscode.workspace) {
      const config = vscode.workspace.getConfiguration('centmem');
      return config.get<string>('binaryPath', 'centmem') || 'centmem';
    }
    return 'centmem';
  }

  public static getWorkspaceCwd(): string | undefined {
    const vscode = getVSCode();
    if (vscode && vscode.workspace && vscode.workspace.workspaceFolders && vscode.workspace.workspaceFolders.length > 0) {
      return vscode.workspace.workspaceFolders[0].uri.fsPath;
    }
    return undefined;
  }

  public static async execute(args: string[]): Promise<string> {
    const binary = this.getBinaryPath();
    const cwd = this.getWorkspaceCwd();

    return new Promise((resolve, reject) => {
      cp.execFile(binary, args, { cwd }, (err, stdout, stderr) => {
        if (err) {
          return reject(parseCLIError(stderr, err.message));
        }
        resolve(stdout.trim());
      });
    });
  }

  public static async recall(query: string, options?: { scope?: string; top?: number; type?: string; tags?: string[] }): Promise<RecallResult> {
    const vscode = getVSCode();
    let scope = options?.scope;
    let top = options?.top;
    if (vscode && vscode.workspace) {
      const config = vscode.workspace.getConfiguration('centmem');
      if (!scope) scope = config.get<string>('scope');
      if (!top) top = config.get<number>('top', 5);
    }

    const args = buildRecallArgs(query, {
      scope,
      top: top || 5,
      type: options?.type,
      tags: options?.tags,
    });

    const stdout = await this.execute(args);
    return JSON.parse(stdout) as RecallResult;
  }

  public static async put(content: string, options?: { scope?: string; type?: string; tags?: string[] }): Promise<PutResult> {
    const vscode = getVSCode();
    let scope = options?.scope;
    if (vscode && vscode.workspace) {
      const config = vscode.workspace.getConfiguration('centmem');
      if (!scope) scope = config.get<string>('scope');
    }

    const args = buildPutArgs(content, {
      scope: scope || 'global',
      type: options?.type,
      tags: options?.tags,
    });

    const stdout = await this.execute(args);
    return JSON.parse(stdout) as PutResult;
  }

  public static async getStats(): Promise<StatsResult> {
    const stdout = await this.execute(['stats']);
    return JSON.parse(stdout) as StatsResult;
  }
}
