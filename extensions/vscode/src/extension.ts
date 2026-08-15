import * as path from "path";
import * as vscode from "vscode";

type PositionSnapshot = {
  line: number;
  character: number;
};

type SelectionSnapshot = {
  anchor: PositionSnapshot;
  active: PositionSnapshot;
};

type EditorFileSnapshot = {
  path: string;
  language_id: string;
  is_dirty: boolean;
  selections: SelectionSnapshot[];
};

type EditorState = {
  schema_version: 1;
  captured_at: string;
  workspace_folder: string | null;
  active_file: string | null;
  open_files: EditorFileSnapshot[];
};

export function activate(context: vscode.ExtensionContext) {
  const captureCommand = vscode.commands.registerCommand("staterelay.captureEditorState", async () => {
    try {
      const state = captureEditorState();
      const outputUri = await writeEditorState(state);
      const label = vscode.workspace.asRelativePath(outputUri);
      vscode.window.showInformationMessage(`StateRelay captured editor state to ${label}`);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      vscode.window.showErrorMessage(message);
    }
  });

  const restoreCommand = vscode.commands.registerCommand("staterelay.restoreEditorState", async () => {
    try {
      const state = await readEditorState();
      const result = await restoreEditorState(state);
      const skipped = result.skipped > 0 ? `, skipped ${result.skipped}` : "";
      vscode.window.showInformationMessage(`StateRelay restored ${result.opened} editor file(s)${skipped}`);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      vscode.window.showErrorMessage(message);
    }
  });

  context.subscriptions.push(captureCommand, restoreCommand);
}

export function deactivate() {}

function captureEditorState(): EditorState {
  const workspaceFolder = firstWorkspaceFolder();
  const activeEditor = vscode.window.activeTextEditor;
  const visibleEditors = new Map<string, vscode.TextEditor>();

  for (const editor of vscode.window.visibleTextEditors) {
    visibleEditors.set(editor.document.uri.toString(), editor);
  }

  const openFiles = vscode.workspace.textDocuments
    .filter((document) => document.uri.scheme === "file")
    .map((document) => snapshotDocument(document, visibleEditors.get(document.uri.toString()), workspaceFolder));

  return {
    schema_version: 1,
    captured_at: new Date().toISOString(),
    workspace_folder: workspaceFolder?.uri.fsPath ?? null,
    active_file: activeEditor ? relativeFilePath(activeEditor.document.uri, workspaceFolder) : null,
    open_files: openFiles,
  };
}

async function writeEditorState(state: EditorState): Promise<vscode.Uri> {
  const workspaceFolder = firstWorkspaceFolder();
  if (!workspaceFolder) {
    throw new Error("StateRelay requires an open workspace folder");
  }

  const directory = vscode.Uri.joinPath(workspaceFolder.uri, ".staterelay");
  const output = vscode.Uri.joinPath(directory, "editor-state.json");
  await vscode.workspace.fs.createDirectory(directory);
  await vscode.workspace.fs.writeFile(output, Buffer.from(JSON.stringify(state, null, 2) + "\n", "utf8"));
  return output;
}

async function readEditorState(): Promise<EditorState> {
  const workspaceFolder = firstWorkspaceFolder();
  if (!workspaceFolder) {
    throw new Error("StateRelay requires an open workspace folder");
  }

  const input = vscode.Uri.joinPath(workspaceFolder.uri, ".staterelay", "editor-state.json");
  const content = await vscode.workspace.fs.readFile(input);
  const parsed = JSON.parse(Buffer.from(content).toString("utf8")) as EditorState;
  validateEditorState(parsed);
  return parsed;
}

async function restoreEditorState(state: EditorState): Promise<{ opened: number; skipped: number }> {
  const workspaceFolder = firstWorkspaceFolder();
  if (!workspaceFolder) {
    throw new Error("StateRelay requires an open workspace folder");
  }

  let opened = 0;
  let skipped = 0;
  const files = orderFilesForRestore(state);

  for (const file of files) {
    const uri = workspaceFileUri(file.path, workspaceFolder);
    if (!uri) {
      skipped++;
      continue;
    }

    try {
      const document = await vscode.workspace.openTextDocument(uri);
      const editor = await vscode.window.showTextDocument(document, { preview: false });
      const selections = file.selections ?? [];
      if (selections.length > 0) {
        editor.selections = selections.map(restoreSelection);
        editor.revealRange(editor.selections[0]);
      }
      opened++;
    } catch {
      skipped++;
    }
  }

  return { opened, skipped };
}

function orderFilesForRestore(state: EditorState): EditorFileSnapshot[] {
  if (!state.active_file) {
    return state.open_files;
  }

  const inactive = state.open_files.filter((file) => file.path !== state.active_file);
  const active = state.open_files.filter((file) => file.path === state.active_file);
  return [...inactive, ...active];
}

function snapshotDocument(
  document: vscode.TextDocument,
  editor: vscode.TextEditor | undefined,
  workspaceFolder: vscode.WorkspaceFolder | undefined,
): EditorFileSnapshot {
  return {
    path: relativeFilePath(document.uri, workspaceFolder),
    language_id: document.languageId,
    is_dirty: document.isDirty,
    selections: editor ? editor.selections.map(snapshotSelection) : [],
  };
}

function snapshotSelection(selection: vscode.Selection): SelectionSnapshot {
  return {
    anchor: snapshotPosition(selection.anchor),
    active: snapshotPosition(selection.active),
  };
}

function snapshotPosition(position: vscode.Position): PositionSnapshot {
  return {
    line: position.line,
    character: position.character,
  };
}

function restoreSelection(selection: SelectionSnapshot): vscode.Selection {
  return new vscode.Selection(restorePosition(selection.anchor), restorePosition(selection.active));
}

function restorePosition(position: PositionSnapshot): vscode.Position {
  return new vscode.Position(Math.max(0, position.line), Math.max(0, position.character));
}

function validateEditorState(state: EditorState): void {
  if (state.schema_version !== 1) {
    throw new Error(`Unsupported StateRelay editor state schema version ${state.schema_version}`);
  }
  if (!Array.isArray(state.open_files)) {
    throw new Error("StateRelay editor state is missing open_files");
  }
}

function firstWorkspaceFolder(): vscode.WorkspaceFolder | undefined {
  return vscode.workspace.workspaceFolders?.[0];
}

function relativeFilePath(uri: vscode.Uri, workspaceFolder: vscode.WorkspaceFolder | undefined): string {
  if (!workspaceFolder) {
    return uri.fsPath;
  }

  return path.relative(workspaceFolder.uri.fsPath, uri.fsPath);
}

function workspaceFileUri(filePath: string, workspaceFolder: vscode.WorkspaceFolder): vscode.Uri | undefined {
  const normalized = filePath.replace(/\\/g, "/");
  if (path.isAbsolute(normalized) || /^[A-Za-z]:\//.test(normalized)) {
    return undefined;
  }

  const parts = normalized.split("/").filter((part) => part.length > 0);
  if (parts.some((part) => part === "." || part === "..")) {
    return undefined;
  }

  return vscode.Uri.joinPath(workspaceFolder.uri, ...parts);
}
