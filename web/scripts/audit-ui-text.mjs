// Diagnostic inventory, not a proof of full translation coverage.
// Run: node web/scripts/audit-ui-text.mjs [filename-fragment]
import { readFileSync, readdirSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import ts from 'typescript'
const root = fileURLToPath(new URL('../src/', import.meta.url))
function files(dir) { return readdirSync(dir, { withFileTypes: true }).flatMap(e => e.isDirectory() ? files(path.join(dir, e.name)) : e.name.endsWith('.tsx') ? [path.join(dir, e.name)] : []) }
let count = 0
for (const file of files(root).filter(f => !process.argv[2] || f.includes(process.argv[2]))) {
  const source = ts.createSourceFile(file, readFileSync(file, 'utf8'), ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX)
  const report = node => {
    const text = node.text?.trim().replace(/\s+/g, ' ')
    if (!text || !/\p{L}{2}/u.test(text)) return
    count++
    const { line } = source.getLineAndCharacterOfPosition(node.getStart(source))
    console.log(`${path.relative(root, file)}:${line + 1}: ${text}`)
  }
  function visit(node) {
    if (ts.isJsxText(node)) report(node)
    if (ts.isJsxAttribute(node) && ['title', 'placeholder', 'aria-label', 'alt'].includes(node.name.getText(source)) && node.initializer && ts.isStringLiteral(node.initializer)) report(node.initializer)
    ts.forEachChild(node, visit)
  }
  visit(source)
}
console.log(`\n${count} candidate literal UI fragments (brands, units and symbols may be intentional).`)
