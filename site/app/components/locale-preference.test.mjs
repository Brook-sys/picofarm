import test from "node:test";
import assert from "node:assert/strict";
import { resolveLocale } from "./locale-preference.mjs";

test("Portuguese language variants select Brazilian Portuguese content", () => {
  for (const language of ["pt", "pt-BR", "pt-PT", "pt-AO", "PT-br"]) {
    assert.equal(resolveLocale(language), "pt-BR");
  }
});

test("other and invalid languages default to English", () => {
  for (const language of ["en-US", "fr", "es", "ptx", "", null, undefined]) {
    assert.equal(resolveLocale(language), "en");
  }
});

test("only supported saved preferences override browser language", () => {
  assert.equal(resolveLocale("pt-PT", "en"), "en");
  assert.equal(resolveLocale("en", "pt-BR"), "pt-BR");
  assert.equal(resolveLocale("pt-PT", "invalid"), "pt-BR");
  assert.equal(resolveLocale("en", "invalid"), "en");
});
