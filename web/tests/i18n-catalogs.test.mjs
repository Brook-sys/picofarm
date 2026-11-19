import assert from 'node:assert/strict'
import { readdirSync, readFileSync } from 'node:fs'
import test from 'node:test'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { createInstance } from 'i18next'

const root = fileURLToPath(new URL('../src/i18n/locales/', import.meta.url))
const read = (locale, ns) => JSON.parse(readFileSync(path.join(root, locale, `${ns}.json`), 'utf8'))
const namespaces = readdirSync(path.join(root, 'en')).filter(f => f.endsWith('.json')).map(f => f.slice(0, -5))
const variables = text => [...text.matchAll(/{{\s*([^},]+)(?:,[^}]+)?\s*}}/g)].map(m => m[1].trim()).sort()

for (const ns of namespaces) {
  test(`${ns}: English and Brazilian Portuguese keys and interpolation match`, () => {
    const en = read('en', ns), pt = read('pt-BR', ns)
    assert.deepEqual(Object.keys(pt).sort(), Object.keys(en).sort())
    for (const key of Object.keys(en)) {
      assert.equal(typeof en[key], 'string', `${ns}:${key}`)
      assert.equal(typeof pt[key], 'string', `${ns}:${key}`)
      assert.ok(en[key].trim(), `Empty English translation: ${ns}:${key}`)
      assert.ok(pt[key].trim(), `Empty Portuguese translation: ${ns}:${key}`)
      assert.deepEqual(variables(pt[key]), variables(en[key]), `Interpolation mismatch: ${ns}:${key}`)
    }
  })
}

test('English catalog values equal their natural-English keys', () => {
  const problems = []
  for (const ns of namespaces) {
    const en = read('en', ns)
    for (const [key, value] of Object.entries(en)) {
      // Plural variants carry an i18next suffix, so their values legitimately
      // differ from the key; only unsuffixed keys repeat the English sentence.
      if (/_(zero|one|two|few|many|other)$/.test(key)) continue
      if (key !== value) problems.push(`${ns}: ${JSON.stringify(key)} => ${JSON.stringify(value)}`)
    }
  }
  assert.deepEqual(problems, [], 'Keep keys as the English sentence itself; only translate other locales.')
})

test('no leftover Portuguese source keys in the English catalogs', () => {
  // Guards the mixed-language failure mode where a Portuguese UI string became
  // the translation key. Heuristic word list, not a language identifier.
  const portuguese = ['falha', 'falhou', 'nenhum', 'nenhuma', 'fatiamento', 'fatiamentos', 'fatiando', 'fatiador', 'cancelar', 'cancelado', 'cancelada', 'cancelando', 'atualizar', 'impressora', 'impressoras', 'impressão', 'impressões', 'arquivo', 'arquivos', 'salvar', 'excluir', 'configuração', 'configurações', 'tarefa', 'tarefas', 'clientes', 'pedidos', 'despesas', 'orçamento', 'orçamentos', 'bobina', 'bobinas', 'carretel', 'abrir', 'importado', 'adicionar', 'remover', 'editar', 'fechar', 'voltar', 'próximo', 'anterior', 'pesquisar', 'filtrar', 'baixar', 'enviar', 'descrição', 'ações', 'opcional', 'obrigatório', 'não', 'usuário', 'senha', 'idioma', 'moeda', 'lucro', 'custo', 'preço', 'quantidade', 'selecione', 'selecionar', 'carregando', 'erro', 'sucesso', 'concluído', 'pausado', 'imprimindo', 'filamento', 'materiais', 'duração', 'iniciar', 'parar', 'retomar', 'exibir', 'ocultar', 'detalhes', 'criar', 'novo', 'nova']
  const pattern = new RegExp(`(?<![\\p{L}])(${portuguese.join('|')})(?![\\p{L}])`, 'iu')
  const problems = []
  for (const ns of namespaces) {
    for (const key of Object.keys(read('en', ns))) {
      const match = key.match(pattern)
      if (match) problems.push(`${ns}: ${JSON.stringify(key)} contains "${match[0]}"`)
    }
  }
  assert.deepEqual(problems, [])
})

test('identical English and pt-BR values are intentional technical terms', () => {
  // Parity alone cannot catch an English value accidentally left in pt-BR.
  // Extend this list only for terms that are genuinely identical in both UI
  // languages (brands, units, protocol names and exact external labels).
  const allowed = {
    commerce: ['SKU:', 'Status', 'Normal', 'Status:', 'SKU', 'Total', 'Design', 'Subtotal', 'Marketplace', 'Etsy', 'Manual'],
    common: ['{{count}}s'],
    components: ['Material', '{{name}} {{value1}}', '{{name}}m'],
    fleet: ['Manual', 'Heatbreak', 'Total', 'total', '{{size}} bytes', 'API', 'URL'],
    library: ['#FFFFFF', 'ABS', 'ASA', 'Item', 'Material', 'Material *', 'Material: {{name}}', 'Offline', 'PETG', 'PLA', 'Prusament', 'Prusament PLA Galaxy Black', 'STL', 'TPU', '{{name}}: {{status}}', '{{count}} G-code', '{{count}} G-code_one', '{{count}} G-code_other', 'G-code'],
    production: [' · {{label}}', 'Bambu Studio', 'BambuStudio', 'Discord', 'Item', 'MakerWorld, Printables, etc.', 'Manual', 'Material', 'SKU:', 'Telegram', 'Total', 'URL', 'Webhook', 'https://...', 'https://discord.com/api/webhooks/...', 'https://www.printables.com/model/...', 'v{{version}}', '{{hours}}h', '{{hours}}h {{minutes}} min', '{{minutes}} min', '{{name}}: {{status}} ({{progress}}%)', '{{type}} {{id}}', '{{value1}}mm'],
    runtime: [],
    settings: ['Backups', 'Orders Read', 'Products Read', 'Tokens', 'Euro (€)'],
  }
  const problems = []
  for (const ns of namespaces) {
    const en = read('en', ns), pt = read('pt-BR', ns)
    const expected = new Set(allowed[ns] || [])
    for (const key of Object.keys(en)) {
      const identical = en[key] === pt[key] && /[a-zA-Z]{3}/.test(en[key])
      if (identical && !expected.has(key)) problems.push(`${ns}: ${JSON.stringify(key)} is still English`)
      if (identical) expected.delete(key)
    }
    for (const key of expected) problems.push(`${ns}: stale identical-value exception ${JSON.stringify(key)}`)
  }
  assert.deepEqual(problems, [])
})

test('catalog keys do not encode plurals with a parenthesized s', () => {
  const problems = []
  for (const ns of namespaces) {
    for (const key of Object.keys(read('en', ns))) {
      if (key.includes('(s)')) problems.push(`${ns}: ${JSON.stringify(key)}`)
    }
  }
  assert.deepEqual(problems, [], 'Use count with _one/_other variants instead.')
})

test('count-based messages use real plural forms in both languages', async () => {
  const i18n = createInstance()
  await i18n.init({
    resources: Object.fromEntries(['en', 'pt-BR'].map(locale => [locale, { components: read(locale, 'components'), library: read(locale, 'library'), production: read(locale, 'production') }])),
    lng: 'en', fallbackLng: 'en', keySeparator: false, nsSeparator: false, interpolation: { escapeValue: false },
  })
  const moved = count => i18n.t('{{count}} item moved to /{{destination}}.', { count, destination: 'sda1', ns: 'components' })
  assert.equal(moved(1), '1 item moved to /sda1.')
  assert.equal(moved(3), '3 items moved to /sda1.')
  await i18n.changeLanguage('pt-BR')
  assert.equal(moved(1), '1 item movido para /sda1.')
  assert.equal(moved(3), '3 itens movidos para /sda1.')
  const uploaded = count => i18n.t('{{count}} file added to the library.', { count, ns: 'library' })
  assert.equal(uploaded(1), '1 arquivo adicionado à biblioteca.')
  assert.equal(uploaded(4), '4 arquivos adicionados à biblioteca.')

  const cases = [
    ['components', 'Delete {{count}} item?', 'Delete 1 item?', 'Delete 2 items?', 'Excluir 1 item?', 'Excluir 2 itens?'],
    ['components', '{{count}} item deleted.', '1 item deleted.', '2 items deleted.', '1 item excluído.', '2 itens excluídos.'],
    ['components', 'Moving {{count}} item', 'Moving 1 item', 'Moving 2 items', 'Movendo 1 item', 'Movendo 2 itens'],
    ['components', 'Snooze for {{count}} hour', 'Snooze for 1 hour', 'Snooze for 2 hours', 'Adiar por 1 hora', 'Adiar por 2 horas'],
    ['library', '{{count}} G-code', '1 G-code', '2 G-codes', '1 G-code', '2 G-codes'],
    ['production', 'Added {{count}} project item to the print queue', 'Added 1 project item to the print queue', 'Added 2 project items to the print queue', 'Adicionado 1 item do projeto à fila de impressão', 'Adicionados 2 itens do projeto à fila de impressão'],
    ['production', 'Added {{count}} file to the queue.', 'Added 1 file to the queue.', 'Added 2 files to the queue.', '1 arquivo adicionado à fila.', '2 arquivos adicionados à fila.'],
    ['production', 'No G-code found. {{count}} part has no valid G-code.', 'No G-code found. 1 part has no valid G-code.', 'No G-code found. 2 parts have no valid G-code.', 'Nenhum G-code encontrado. 1 peça não tem G-code válido.', 'Nenhum G-code encontrado. 2 peças não têm G-code válido.'],
    ['production', '{{count}} item added to the print queue.', '1 item added to the print queue.', '2 items added to the print queue.', '1 item adicionado à fila de impressão.', '2 itens adicionados à fila de impressão.'],
    ['production', '{{count}} part does not have valid G-code.', '1 part does not have valid G-code.', '2 parts do not have valid G-code.', '1 peça não tem G-code válido.', '2 peças não têm G-code válido.'],
    ['production', '{{count}} sale', '1 sale', '2 sales', '1 venda', '2 vendas'],
    ['production', '{{count}} STL file detected', '1 STL file detected', '2 STL files detected', '1 arquivo STL detectado', '2 arquivos STL detectados'],
    ['production', 'This task still has {{count}} print job. The database blocks deleting a task with jobs — remove the jobs first.', 'This task still has 1 print job. The database blocks deleting a task with jobs — remove the jobs first.', 'This task still has 2 print jobs. The database blocks deleting a task with jobs — remove the jobs first.', 'Esta tarefa ainda tem 1 trabalho de impressão. O banco de dados impede excluir uma tarefa com trabalhos — remova os trabalhos primeiro.', 'Esta tarefa ainda tem 2 trabalhos de impressão. O banco de dados impede excluir uma tarefa com trabalhos — remova os trabalhos primeiro.'],
  ]
  for (const [locale, singular, plural] of [['en', 2, 3], ['pt-BR', 4, 5]]) {
    await i18n.changeLanguage(locale)
    for (const row of cases) {
      const [ns, key] = row
      assert.equal(i18n.t(key, { ns, count: 1 }), row[singular], `${locale}/${ns}/${key} singular`)
      assert.equal(i18n.t(key, { ns, count: 2 }), row[plural], `${locale}/${ns}/${key} plural`)
    }
  }
})

test('bundled catalogs switch languages without changing runtime values', async () => {
  const resources = Object.fromEntries(['en', 'pt-BR'].map(locale => [locale,
    Object.fromEntries(namespaces.map(ns => [ns, read(locale, ns)]))]))
  const i18n = createInstance()
  await i18n.init({ resources, lng: 'en', fallbackLng: 'en', keySeparator: false, nsSeparator: false, interpolation: { escapeValue: false } })
  const key = 'A print job finished.'
  assert.equal(i18n.t(key, { ns: 'runtime' }), 'A print job finished.')
  await i18n.changeLanguage('pt-BR')
  assert.equal(i18n.t(key, { ns: 'runtime' }), 'Uma impressão foi concluída.')
  await i18n.changeLanguage('unsupported')
  assert.equal(i18n.t(key, { ns: 'runtime' }), 'A print job finished.')
})
