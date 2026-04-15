<script setup lang="ts">
import type { DocSymbol, DocParam } from "~/types/api";

const props = defineProps<{
  symbol: DocSymbol;
}>();

interface Token {
  text: string;
  cls: string;
}

function punct(text: string): Token {
  return { text, cls: "py-punct" };
}

function buildFunctionTokens(symbol: DocSymbol): Token[] {
  const tokens: Token[] = [];

  if (symbol.signature.startsWith("async ")) {
    tokens.push({ text: "async", cls: "py-keyword" });
    tokens.push({ text: " ", cls: "" });
  }

  tokens.push({ text: "def", cls: "py-keyword" });
  tokens.push({ text: " ", cls: "" });
  tokens.push({ text: symbol.name, cls: "py-name" });
  tokens.push(punct("("));

  const params = symbol.parameters ?? [];

  // Track whether we've inserted the `/` separator (after all positional-only params)
  let insertedSlash = false;
  // Track whether we've inserted the `*` separator (before keyword-only params)
  let insertedStar = false;

  const positionalOnlyParams = params.filter((p) => p.kind === "positional_only");
  const hasPositionalOnly = positionalOnlyParams.length > 0;
  const hasKeywordOnly = params.some((p) => p.kind === "keyword_only");
  const hasVarPositional = params.some((p) => p.kind === "var_positional");

  let first = true;

  for (let i = 0; i < params.length; i++) {
    const param = params[i];

    // After all positional_only params, insert `/`
    if (hasPositionalOnly && !insertedSlash && param.kind !== "positional_only") {
      if (!first) tokens.push(punct(", "));
      tokens.push(punct("/"));
      insertedSlash = true;
      first = false;
    }

    // Before keyword_only params, insert `*` separator (only if no var_positional already)
    if (hasKeywordOnly && !insertedStar && !hasVarPositional && param.kind === "keyword_only") {
      if (!first) tokens.push(punct(", "));
      tokens.push(punct("*"));
      insertedStar = true;
      first = false;
    }

    if (!first) tokens.push(punct(", "));
    first = false;

    // Prefix for var_positional and var_keyword
    if (param.kind === "var_positional") {
      tokens.push(punct("*"));
      insertedStar = true;
    } else if (param.kind === "var_keyword") {
      tokens.push(punct("**"));
    }

    tokens.push({ text: param.name, cls: "py-param" });

    if (param.type) {
      tokens.push(punct(": "));
      tokens.push({ text: param.type, cls: "py-type" });
    }

    if (param.default !== undefined) {
      tokens.push(punct(" = "));
      tokens.push({ text: param.default, cls: "py-default" });
    }
  }

  // Handle trailing `/` if all params were positional_only
  if (hasPositionalOnly && !insertedSlash) {
    if (!first) tokens.push(punct(", "));
    tokens.push(punct("/"));
  }

  tokens.push(punct(")"));

  if (symbol.returns?.type) {
    tokens.push({ text: " -> ", cls: "py-punct" });
    tokens.push({ text: symbol.returns.type, cls: "py-type" });
  }

  return tokens;
}

function buildClassTokens(symbol: DocSymbol): Token[] {
  const tokens: Token[] = [];

  tokens.push({ text: "class", cls: "py-keyword" });
  tokens.push({ text: " ", cls: "" });
  tokens.push({ text: symbol.name, cls: "py-name" });

  // Extract base classes from signature string
  const match = symbol.signature.match(/\(([^)]*)\)/);
  if (match && match[1].trim()) {
    tokens.push(punct("("));
    const bases = match[1]
      .split(",")
      .map((b) => b.trim())
      .filter(Boolean);
    bases.forEach((base, idx) => {
      if (idx > 0) tokens.push(punct(", "));
      tokens.push({ text: base, cls: "py-type" });
    });
    tokens.push(punct(")"));
  }

  return tokens;
}

const tokens = computed<Token[]>(() => {
  if (props.symbol.kind === "class" || props.symbol.kind === "exception") {
    return buildClassTokens(props.symbol);
  }
  return buildFunctionTokens(props.symbol);
});
</script>

<template>
  <div
    class="rounded-md border border-zinc-800 bg-zinc-900 px-4 py-2.5 font-mono text-[11px] leading-relaxed overflow-x-auto"
  >
    <span v-for="(token, i) in tokens" :key="i" :class="token.cls || undefined">{{
      token.text
    }}</span>
  </div>
</template>

<style scoped>
.py-keyword {
  color: var(--py-keyword);
}
.py-name {
  color: var(--py-name);
}
.py-param {
  color: var(--py-param);
}
.py-type {
  color: var(--py-type);
}
.py-default {
  color: var(--py-default);
}
.py-punct {
  color: var(--py-punct);
}
.py-decorator {
  color: var(--py-decorator);
}
</style>
