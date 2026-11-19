import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const icon = readFileSync(new URL('./github-icon.tsx', import.meta.url), 'utf8');
const page = readFileSync(new URL('../page.tsx', import.meta.url), 'utf8');

test('local GitHub brand mark is decorative and inherits link color', () => {
  assert.ok(icon.includes('<svg'));
  assert.ok(icon.includes('<path d='));
  assert.ok(icon.includes('fill="currentColor"'));
  assert.ok(icon.includes('aria-hidden="true"'));
  assert.ok(icon.includes('focusable="false"'));
  assert.ok(icon.includes('className={className}'));
});

test('both GitHub links retain visible accessible text without the removed Lucide export', () => {
  assert.doesNotMatch(page, /\bGithub\b/);
  assert.equal((page.match(/<GitHubIcon className="h-4 w-4" \/>\{t.github\}/g) ?? []).length, 2);
  assert.ok(page.includes('import { GitHubIcon } from "./components/github-icon"'));
});
