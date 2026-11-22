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
  if (name === 'react-i18next') return { useTranslation: () => ({ t: key => key }) }
  if (name === './MaterialBadge') return { MaterialBadge: () => null }
  throw new Error(`Unexpected dependency: ${name}`)
} }
vm.runInNewContext(outputText, context)
const render = file => JSON.stringify(context.exports.default({ file, colors: {} }))

test('visible filament summary prefers the same slicer profile as file details', () => {
  const output = render({ material_type: 'PLA', filament_name: 'Legacy spool', metadata: { filament_settings_id: 'Generic PLA @ printer' } })
  assert.ok(output.includes('Generic PLA @ printer'))
  assert.ok(output.includes('PLA'))
  assert.ok(!output.includes('Legacy spool'))
})

test('legacy filament name remains visible and missing metadata is explicit', () => {
  assert.ok(render({ filament_name: 'PETG Blue' }).includes('PETG Blue'))
  assert.ok(render({ filament_name: 'Legacy', metadata: { filament_settings_id: '  ' } }).includes('Legacy'))
  assert.ok(render({}).includes('Not found'))
  assert.ok(render({ metadata: {} }).includes('Not found'))
})

test('long and untrusted profile labels are rendered as wrapping text, not HTML', () => {
  const output = render({ metadata: { filament_settings_id: '<script>unsafe</script>' } })
  assert.ok(output.includes('<script>unsafe</script>'))
  assert.ok(output.includes('overflow-wrap:anywhere'))
  assert.ok(!output.includes('dangerouslySetInnerHTML'))
})
