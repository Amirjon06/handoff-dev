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
  const command = vscode.commands.registerCommand("staterelay.captureEditorState", async () => {
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

  context.subscriptions.push(command);
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

function firstWorkspaceFolder(): vscode.WorkspaceFolder | undefined {
  return vscode.workspace.workspaceFolders?.[0];
}

function relativeFilePath(uri: vscode.Uri, workspaceFolder: vscode.WorkspaceFolder | undefined): string {
  if (!workspaceFolder) {
    return uri.fsPath;
  }

  return path.relative(workspaceFolder.uri.fsPath, uri.fsPath);
}
