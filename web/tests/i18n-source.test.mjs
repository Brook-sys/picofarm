import assert from 'node:assert/strict'
import { readFileSync, readdirSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import test from 'node:test'
import ts from 'typescript'

const src = fileURLToPath(new URL('../src/', import.meta.url))
function files(dir) {
  return readdirSync(dir, { withFileTypes: true }).flatMap(item => item.isDirectory() ? files(path.join(dir, item.name)) : /\.tsx?$/.test(item.name) ? [path.join(dir, item.name)] : [])
}

test('native dialogs never use raw literal strings', () => {
  const problems = []
  for (const file of files(src)) {
    const source = ts.createSourceFile(file, readFileSync(file, 'utf8'), ts.ScriptTarget.Latest, true, file.endsWith('.tsx') ? ts.ScriptKind.TSX : ts.ScriptKind.TS)
    function check(node) {
      if (ts.isCallExpression(node)) {
        const callee = node.expression.getText(source)
        if (['alert', 'window.alert', 'confirm', 'window.confirm', 'prompt', 'window.prompt'].includes(callee)) {
          const first = node.arguments[0]
          if (first && (ts.isStringLiteral(first) || ts.isNoSubstitutionTemplateLiteral(first) || ts.isTemplateExpression(first))) {
            const { line } = source.getLineAndCharacterOfPosition(node.getStart(source))
            problems.push(`${path.relative(src, file)}:${line + 1}: ${callee}("${first.text}")`)
          }
        }
      }
      ts.forEachChild(node, check)
    }
    check(source)
  }
  assert.deepEqual(problems, [], 'Translate dialog text at the call site.')
})

test('literal translation calls have entries in both bundled languages', () => {
  const problems = []
  for (const file of files(src).filter(f => !f.includes(`${path.sep}i18n${path.sep}`))) {
    const source = ts.createSourceFile(file, readFileSync(file, 'utf8'), ts.ScriptTarget.Latest, true, file.endsWith('.tsx') ? ts.ScriptKind.TSX : ts.ScriptKind.TS)
    let namespace
    function findNamespace(node) {
      if (ts.isCallExpression(node) && node.expression.getText(source) === 'useTranslation' && node.arguments[0] && ts.isStringLiteral(node.arguments[0])) namespace = node.arguments[0].text
      ts.forEachChild(node, findNamespace)
    }
    findNamespace(source)
    function check(node) {
      if (ts.isCallExpression(node) && ['t', 'i18n.t'].includes(node.expression.getText(source)) && node.arguments[0] && ts.isStringLiteral(node.arguments[0])) {
        let ns = namespace
        const options = node.arguments[1]
        if (options && ts.isObjectLiteralExpression(options)) {
          for (const property of options.properties) if (ts.isPropertyAssignment(property) && property.name.getText(source) === 'ns' && ts.isStringLiteral(property.initializer)) ns = property.initializer.text
        }
        if (ns) for (const locale of ['en', 'pt-BR']) {
          const catalog = JSON.parse(readFileSync(path.join(src, 'i18n', 'locales', locale, `${ns}.json`), 'utf8'))
          if (!(node.arguments[0].text in catalog)) {
            const position = source.getLineAndCharacterOfPosition(node.getStart(source))
            problems.push(`${path.relative(src, file)}:${position.line + 1} missing ${locale}/${ns}: ${node.arguments[0].text}`)
          }
        }
      }
      ts.forEachChild(node, check)
    }
    check(source)
  }
  assert.deepEqual(problems, [])
})
