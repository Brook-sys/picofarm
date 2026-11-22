import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import vm from 'node:vm'
import ts from 'typescript'

const source = readFileSync(new URL('../src/components/GCodeFilamentSummary.tsx', import.meta.url), 'utf8')
const { outputText } = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX } })
const jsx = (type, props) => ({ type: typeof type === 'string' ? type : 'MaterialBadge', ...props })
const context = { exports: {}, require: name => {
  if (name === 'react/jsx-runtime') return { jsx, jsxs: jsx }
  if (name === './MaterialBadge') return { MaterialBadge: () => null }
  throw new Error(`Unexpected dependency: ${name}`)
} }
vm.runInNewContext(outputText, context)
const render = (file, colors = {}) => context.exports.default({ file, colors })

test('G-code summary shows only the material badge, not filament profile or name', () => {
  const colors = { pla: '#ffffff' }
  const result = render({ material_type: 'PLA', filament_name: 'Legacy spool', metadata: { filament_settings_id: 'Generic PLA @ printer' } }, colors)
  assert.equal(result.children.type, 'MaterialBadge')
  assert.equal(result.children.material, 'PLA')
  assert.equal(result.children.colors, colors)
  assert.ok(!JSON.stringify(result).includes('Legacy spool'))
  assert.ok(!JSON.stringify(result).includes('Generic PLA @ printer'))
  assert.ok(!JSON.stringify(result).includes('Filament Profile'))
})

test('missing material delegates the existing unknown-material presentation to the badge', () => {
  assert.equal(render({}).children.material, undefined)
})
