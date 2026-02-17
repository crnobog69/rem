# Remfile VS Code Extension

Language support for `Remfile`:

- file detection (`Remfile`)
- syntax highlighting (TOML)
- comment and bracket behavior
- starter snippets
- lightweight diagnostics:
- duplicate task names
- invalid top-level/task statements
- missing/invalid default target
- undefined dependencies

## Local usage

1. Open `vscode/remfile` in VS Code.
2. Press `F5` to run Extension Development Host.
3. Open any `Remfile` and select language `Remfile` if needed.

## Package as VSIX (optional)

```bash
cd vscode/remfile
npx @vscode/vsce package
```
