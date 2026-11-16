import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, readdirSync } from 'node:fs';
import sharp from 'sharp';
const content = readFileSync(new URL('./landing-content.ts', import.meta.url), 'utf8');
const { landingContent } = await import(`data:text/javascript;base64,${Buffer.from(content).toString('base64')}`);

const directory = new URL('../../public/screenshots/', import.meta.url);
const captures = [
  'tela_geral_dashboardprincipal', 'tela_files', 'tela_cameras',
  'tela_printerfiles', 'tela_fatiamento', 'tela_filamentos',
  'tela_notifications', 'tela_template_notifications',
];

test('only PicoFarm captures remain, with optimized WebP counterparts', async () => {
  const files = readdirSync(directory);
  assert.equal(files.length, captures.length * 2);
  for (const name of captures) {
    for (const extension of ['png', 'webp']) assert.ok(files.includes(`${name}.${extension}`));
    const image = await sharp(new URL(`${name}.webp`, directory).pathname).metadata();
    assert.equal(image.width, 1600);
    assert.equal(image.format, 'webp');
  }
});

test('both locales describe the same gallery without placeholder copy', () => {
  assert.deepEqual(landingContent['pt-BR'].gallery.map(([file]) => file), landingContent.en.gallery.map(([file]) => file));
  for (const locale of Object.values(landingContent)) {
    assert.equal(locale.gallery.length, 5);
    for (const [file, title, description] of locale.gallery) {
      assert.ok(captures.includes(file));
      assert.ok(title.length > 0 && description.length > 0);
    }
    assert.ok(!('placeholder' in locale));
  }
});

test('printer-files source has no residual connection value pixels', async () => {
  const { data, info } = await sharp(new URL('tela_printerfiles.png', directory).pathname)
    .extract({ left: 888, top: 259, width: 205, height: 37 })
    .raw().toBuffer({ resolveWithObject: true });
  for (let i = 0; i < data.length; i += info.channels) {
    assert.deepEqual([...data.subarray(i, i + 3)], [20, 20, 22]);
  }
});

test('landing uses real images with lazy loading and no placeholder component', () => {
  const page = readFileSync(new URL('../page.tsx', import.meta.url), 'utf8');
  assert.ok(!page.includes('ScreenshotPlaceholder'));
  assert.ok(page.includes('loading={eager ? "eager" : "lazy"}'));
  for (const name of captures.slice(0, 3)) assert.ok(page.includes(name));
});
