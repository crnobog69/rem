const vscode = require("vscode");

function activate(context) {
  const collection = vscode.languages.createDiagnosticCollection("remfile");
  context.subscriptions.push(collection);

  const refresh = (doc) => {
    if (!isRemfileDocument(doc)) {
      return;
    }
    const enabled = vscode.workspace
      .getConfiguration("remfile", doc.uri)
      .get("diagnostics.enabled", true);
    if (!enabled) {
      collection.delete(doc.uri);
      return;
    }
    collection.set(doc.uri, validate(doc));
  };

  context.subscriptions.push(
    vscode.workspace.onDidOpenTextDocument(refresh),
    vscode.workspace.onDidSaveTextDocument(refresh),
    vscode.workspace.onDidCloseTextDocument((doc) => collection.delete(doc.uri)),
    vscode.workspace.onDidChangeTextDocument((event) => refresh(event.document))
  );

  vscode.workspace.textDocuments.forEach(refresh);
  if (vscode.window.activeTextEditor) {
    refresh(vscode.window.activeTextEditor.document);
  }
}

function deactivate() {}

function isRemfileDocument(doc) {
  if (!doc) {
    return false;
  }
  if (doc.languageId === "remfile") {
    return true;
  }
  const name = doc.fileName.split(/[\\/]/).pop();
  return name === "Remfile";
}

function validate(doc) {
  const lines = doc.getText().split(/\r?\n/);
  return validateToml(doc, lines).diagnostics;
}

function validateToml(doc, lines) {
  const diagnostics = [];
  const tasks = new Map();
  const vars = new Set();
  const taskDeps = new Map();
  let defaultTarget = "";
  let parsed = false;

  let section = "root";
  let currentTask = null;
  let inArray = false;
  let arrayBalance = 0;

  for (let i = 0; i < lines.length; i++) {
    const raw = lines[i];
    let line = stripInlineComment(raw).trim();
    if (!line) {
      continue;
    }

    if (inArray) {
      arrayBalance += bracketDelta(line);
      if (arrayBalance <= 0) {
        inArray = false;
        arrayBalance = 0;
      }
      continue;
    }

    const sectionMatch = line.match(/^\[\s*([^\]]+)\s*\]$/);
    if (sectionMatch) {
      parsed = true;
      const name = sectionMatch[1].trim();
      if (name === "vars") {
        section = "vars";
        currentTask = null;
      } else if (name.startsWith("task.")) {
        const taskName = name.slice("task.".length).trim();
        if (!/^[A-Za-z0-9_.-]+$/.test(taskName)) {
          diagnostics.push(diag(doc, i, raw.length, `invalid task name "${taskName}"`));
        } else if (tasks.has(taskName)) {
          diagnostics.push(diag(doc, i, raw.length, `duplicate task "${taskName}"`));
        } else {
          tasks.set(taskName, i);
          taskDeps.set(taskName, []);
          section = "task";
          currentTask = taskName;
        }
      } else {
        diagnostics.push(diag(doc, i, raw.length, `unsupported section "${name}"`));
      }
      continue;
    }

    const idx = line.indexOf("=");
    if (idx < 0) {
      diagnostics.push(diag(doc, i, raw.length, "invalid TOML statement (expected key = value)"));
      continue;
    }
    parsed = true;

    const key = line.slice(0, idx).trim();
    const value = line.slice(idx + 1).trim();

    if (value.startsWith("[")) {
      arrayBalance = bracketDelta(value);
      if (arrayBalance > 0) {
        inArray = true;
      }
    }

    if (section === "root") {
      if (key === "default") {
        defaultTarget = unquote(value).trim();
        if (!defaultTarget) {
          diagnostics.push(diag(doc, i, raw.length, "default target name is missing"));
        }
      } else {
        diagnostics.push(diag(doc, i, raw.length, `unsupported top-level key "${key}"`));
      }
      continue;
    }

    if (section === "vars") {
      if (!/^[A-Za-z_][A-Za-z0-9_]*$/.test(key)) {
        diagnostics.push(diag(doc, i, raw.length, `invalid var name "${key}"`));
      } else if (vars.has(key)) {
        diagnostics.push(diag(doc, i, raw.length, `duplicate var "${key}"`));
      } else {
        vars.add(key);
      }
      continue;
    }

    if (section === "task" && currentTask) {
      const allowed = new Set(["desc", "deps", "inputs", "outputs", "cmd", "cmds", "dir"]);
      if (!allowed.has(key)) {
        diagnostics.push(diag(doc, i, raw.length, `unknown task field "${key}"`));
        continue;
      }
      if (key === "deps") {
        taskDeps.set(currentTask, parseListValue(value));
      }
      if ((key === "cmd" || key === "cmds") && value === "") {
        diagnostics.push(diag(doc, i, raw.length, "empty command value"));
      }
    }
  }

  if (!parsed) {
    return { parsed: false, diagnostics };
  }

  if (tasks.size === 0) {
    diagnostics.push(diag(doc, 0, 0, "Remfile has no tasks"));
    return { parsed: true, diagnostics };
  }

  if (defaultTarget && !tasks.has(defaultTarget)) {
    diagnostics.push(
      diag(doc, 0, 0, `default target "${defaultTarget}" does not match any defined task`)
    );
  }

  for (const [taskName, deps] of taskDeps.entries()) {
    deps.forEach((dep) => {
      if (!dep) {
        return;
      }
      if (!tasks.has(dep) && !/\$\{.+\}/.test(dep)) {
        const taskLine = tasks.get(taskName) || 0;
        diagnostics.push(
          diag(
            doc,
            taskLine,
            lines[taskLine] ? lines[taskLine].length : 0,
            `task "${taskName}" depends on undefined task "${dep}"`
          )
        );
      }
    });
  }

  return { parsed: true, diagnostics };
}

function parseListValue(value) {
  if (!value) {
    return [];
  }
  const trimmed = value.trim();
  if (!trimmed.startsWith("[")) {
    return splitList(unquote(trimmed));
  }
  const inner = trimmed.replace(/^\[/, "").replace(/\]$/, "");
  return inner
    .split(",")
    .map((part) => unquote(part.trim()))
    .filter(Boolean)
    .flatMap((part) => splitList(part));
}

function splitList(value) {
  return value
    .replace(/^\[/, "")
    .replace(/\]$/, "")
    .split(/[\s,]+/)
    .map((part) => part.trim())
    .filter(Boolean);
}

function unquote(s) {
  if (!s || s.length < 2) {
    return s;
  }
  if ((s.startsWith('"') && s.endsWith('"')) || (s.startsWith("'") && s.endsWith("'"))) {
    return s.slice(1, -1);
  }
  return s;
}

function stripInlineComment(s) {
  let inSingle = false;
  let inDouble = false;
  let escaped = false;

  for (let i = 0; i < s.length; i++) {
    const c = s[i];
    if (inDouble) {
      if (escaped) {
        escaped = false;
        continue;
      }
      if (c === "\\") {
        escaped = true;
        continue;
      }
      if (c === '"') {
        inDouble = false;
      }
      continue;
    }
    if (inSingle) {
      if (c === "'") {
        inSingle = false;
      }
      continue;
    }

    if (c === '"') {
      inDouble = true;
      continue;
    }
    if (c === "'") {
      inSingle = true;
      continue;
    }
    if (c === "#") {
      return s.slice(0, i);
    }
  }
  return s;
}

function bracketDelta(s) {
  let delta = 0;
  let inSingle = false;
  let inDouble = false;
  let escaped = false;

  for (let i = 0; i < s.length; i++) {
    const c = s[i];
    if (inDouble) {
      if (escaped) {
        escaped = false;
        continue;
      }
      if (c === "\\") {
        escaped = true;
        continue;
      }
      if (c === '"') {
        inDouble = false;
      }
      continue;
    }
    if (inSingle) {
      if (c === "'") {
        inSingle = false;
      }
      continue;
    }

    if (c === '"') {
      inDouble = true;
      continue;
    }
    if (c === "'") {
      inSingle = true;
      continue;
    }
    if (c === "[") {
      delta += 1;
      continue;
    }
    if (c === "]") {
      delta -= 1;
    }
  }
  return delta;
}

function diag(doc, line, endChar, message) {
  const l = Math.max(0, Math.min(line, doc.lineCount - 1));
  const range = new vscode.Range(
    new vscode.Position(l, 0),
    new vscode.Position(l, Math.max(0, endChar))
  );
  return new vscode.Diagnostic(range, message, vscode.DiagnosticSeverity.Warning);
}

module.exports = {
  activate,
  deactivate,
};
