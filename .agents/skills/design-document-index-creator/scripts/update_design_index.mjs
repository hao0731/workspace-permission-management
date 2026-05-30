#!/usr/bin/env node

import {
  existsSync,
  mkdirSync,
  readdirSync,
  readFileSync,
  statSync,
  writeFileSync,
} from "node:fs";
import {
  basename,
  dirname,
  isAbsolute,
  join,
  relative,
  resolve,
  sep,
} from "node:path";
import process from "node:process";

const REQUIRED_FIELDS = ["doc_id", "title", "status", "code_paths", "tags", "summary"];
const INDEX_FIELDS = ["doc_id", "path", "title", "status", "code_paths", "tags", "summary"];

class IndexErrorMessage extends Error {}

function main(argv = process.argv.slice(2)) {
  try {
    const args = parseArgs(argv);
    if (args.help) {
      printHelp();
      return 0;
    }

    const repoRoot = resolve(args.repoRoot);
    const updates = collectUpdates(repoRoot, args.documents, args.scan);
    if (updates.length === 0) {
      throw new IndexErrorMessage("No design documents found to index.");
    }

    const groups = groupUpdates(repoRoot, updates, args.indexPath);
    for (const [indexPath, docs] of groups.entries()) {
      const existing = readIndex(indexPath);
      const merged = mergeDocuments(existing, docs);
      writeIndex(indexPath, merged);
      console.log(`Updated ${relativePath(indexPath, repoRoot)} (${merged.length} documents)`);
    }
    return 0;
  } catch (error) {
    if (error instanceof IndexErrorMessage) {
      console.error(error.message);
      return 1;
    }
    throw error;
  }
}

function parseArgs(argv) {
  const args = {
    documents: [],
    scan: [],
    indexPath: null,
    repoRoot: ".",
    help: false,
  };

  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index];
    if (arg === "-h" || arg === "--help") {
      args.help = true;
      continue;
    }
    if (arg === "--scan") {
      args.scan.push(readOptionValue(argv, index, "--scan"));
      index += 1;
      continue;
    }
    if (arg === "--index-path") {
      args.indexPath = readOptionValue(argv, index, "--index-path");
      index += 1;
      continue;
    }
    if (arg === "--repo-root") {
      args.repoRoot = readOptionValue(argv, index, "--repo-root");
      index += 1;
      continue;
    }
    if (arg.startsWith("-")) {
      throw new IndexErrorMessage(`Unknown option: ${arg}`);
    }
    args.documents.push(arg);
  }

  if (args.documents.length === 0 && args.scan.length === 0) {
    args.scan.push("docs/designs");
  }
  return args;
}

function readOptionValue(argv, index, option) {
  const value = argv[index + 1];
  if (!value || value.startsWith("-")) {
    throw new IndexErrorMessage(`Missing value for ${option}`);
  }
  return value;
}

function printHelp() {
  console.log(`Usage: update_design_index.mjs [options] [documents...]

Create or update index.yaml from design-document frontmatter.

Options:
  --scan DIR        Scan a design-doc directory for Markdown files. May be repeated.
  --index-path PATH Write all discovered documents to this index.yaml path.
  --repo-root DIR   Repository root used for relative paths. Defaults to current directory.
  -h, --help        Show this help.
`);
}

function collectUpdates(repoRoot, documentArgs, scanArgs) {
  const updates = [];

  for (const document of documentArgs) {
    const path = resolveUnderRoot(repoRoot, document);
    const entry = documentEntry(repoRoot, path);
    entry._indexPath = join(dirname(path), "index.yaml");
    updates.push(entry);
  }

  for (const scanDir of scanArgs) {
    const root = resolveUnderRoot(repoRoot, scanDir);
    if (!existsSync(root)) {
      throw new IndexErrorMessage(`Scan directory not found: ${scanDir}`);
    }
    if (!statSync(root).isDirectory()) {
      throw new IndexErrorMessage(`Scan path is not a directory: ${scanDir}`);
    }

    for (const path of findMarkdownFiles(root)) {
      if (basename(path).toLowerCase() === "index.md") {
        continue;
      }
      const entry = documentEntry(repoRoot, path);
      entry._indexPath = join(root, "index.yaml");
      updates.push(entry);
    }
  }

  const seen = new Set();
  const uniqueUpdates = [];
  for (const update of updates) {
    const key = `${update.path}\0${update._indexPath}`;
    if (seen.has(key)) {
      continue;
    }
    seen.add(key);
    uniqueUpdates.push(update);
  }
  return uniqueUpdates;
}

function findMarkdownFiles(root) {
  const files = [];
  const entries = readdirSync(root, { withFileTypes: true }).sort((left, right) =>
    left.name.localeCompare(right.name),
  );

  for (const entry of entries) {
    const path = join(root, entry.name);
    if (entry.isDirectory()) {
      files.push(...findMarkdownFiles(path));
    } else if (entry.isFile() && entry.name.toLowerCase().endsWith(".md")) {
      files.push(path);
    }
  }
  return files;
}

function groupUpdates(repoRoot, updates, indexArg) {
  const groups = new Map();
  if (indexArg) {
    groups.set(
      resolveUnderRoot(repoRoot, indexArg),
      updates.map((update) => publicDocument(update)),
    );
    return groups;
  }

  for (const update of updates) {
    const indexPath = update._indexPath;
    if (!groups.has(indexPath)) {
      groups.set(indexPath, []);
    }
    groups.get(indexPath).push(publicDocument(update));
  }
  return groups;
}

function publicDocument(update) {
  return Object.fromEntries(INDEX_FIELDS.map((field) => [field, update[field]]));
}

function documentEntry(repoRoot, path) {
  if (!existsSync(path)) {
    throw new IndexErrorMessage(`Design document not found: ${relativePath(path, repoRoot)}`);
  }
  if (!statSync(path).isFile()) {
    throw new IndexErrorMessage(`Design document is not a file: ${relativePath(path, repoRoot)}`);
  }

  const frontmatter = parseFrontmatter(path);
  const missing = REQUIRED_FIELDS.filter((field) => !Object.hasOwn(frontmatter, field));
  if (missing.length > 0) {
    throw new IndexErrorMessage(
      `Missing required frontmatter fields in ${relativePath(path, repoRoot)}: ${missing.join(", ")}`,
    );
  }

  const entry = {
    doc_id: asText(frontmatter.doc_id),
    path: relativePath(path, repoRoot),
    title: asText(frontmatter.title),
    status: asText(frontmatter.status),
    code_paths: asList(frontmatter.code_paths),
    tags: asList(frontmatter.tags),
    summary: asText(frontmatter.summary),
  };

  const empty = ["doc_id", "title", "status", "summary"].filter((field) => !entry[field]);
  if (empty.length > 0) {
    throw new IndexErrorMessage(
      `Required frontmatter fields must not be empty in ${relativePath(path, repoRoot)}: ${empty.join(", ")}`,
    );
  }
  return entry;
}

function parseFrontmatter(path) {
  const lines = readFileSync(path, "utf8").split(/\r?\n/);
  if (lines.length === 0 || lines[0].trim() !== "---") {
    throw new IndexErrorMessage(`Missing YAML frontmatter in ${path}`);
  }

  const end = lines.findIndex((line, index) => index > 0 && line.trim() === "---");
  if (end === -1) {
    throw new IndexErrorMessage(`Unclosed YAML frontmatter in ${path}`);
  }

  return parseSimpleYaml(lines.slice(1, end));
}

function parseSimpleYaml(lines) {
  const result = {};
  let index = 0;

  while (index < lines.length) {
    const raw = lines[index];
    const trimmed = raw.trim();
    if (!trimmed || raw.trimStart().startsWith("#") || raw.startsWith(" ")) {
      index += 1;
      continue;
    }
    if (!raw.includes(":")) {
      index += 1;
      continue;
    }

    const colon = raw.indexOf(":");
    const key = raw.slice(0, colon).trim();
    const value = raw.slice(colon + 1).trim();

    if ([">", "|", ">-", "|-", ">+", "|+"].includes(value)) {
      const [block, nextIndex] = readBlock(lines, index + 1);
      result[key] = block;
      index = nextIndex;
      continue;
    }

    if (value === "") {
      const [nested, nextIndex] = readNestedValue(lines, index + 1);
      result[key] = nested;
      index = nextIndex;
      continue;
    }

    result[key] = parseScalarOrInlineList(value);
    index += 1;
  }

  return result;
}

function readBlock(lines, start) {
  const blockLines = [];
  let index = start;
  while (index < lines.length) {
    const line = lines[index];
    if (line && !line.startsWith(" ")) {
      break;
    }
    const trimmed = line.trim();
    if (trimmed) {
      blockLines.push(trimmed);
    }
    index += 1;
  }
  return [blockLines.join(" ").trim(), index];
}

function readNestedValue(lines, start) {
  const items = [];
  let index = start;
  while (index < lines.length) {
    const line = lines[index];
    const trimmed = line.trim();
    if (line && !line.startsWith(" ")) {
      break;
    }
    if (trimmed.startsWith("- ")) {
      items.push(unquote(trimmed.slice(2).trim()));
    }
    index += 1;
  }
  return [items, index];
}

function parseScalarOrInlineList(value) {
  if (value === "[]") {
    return [];
  }
  if (value.startsWith("[") && value.endsWith("]")) {
    const inner = value.slice(1, -1).trim();
    if (!inner) {
      return [];
    }
    return inner.split(",").map((part) => unquote(part.trim()));
  }
  return unquote(value);
}

function readIndex(path) {
  if (!existsSync(path)) {
    return [];
  }

  const lines = readFileSync(path, "utf8").split(/\r?\n/);
  const documents = [];
  let current = null;
  let index = 0;

  while (index < lines.length) {
    const line = lines[index];
    const trimmed = line.trim();
    if (line.startsWith("  - doc_id:")) {
      current = { doc_id: unquote(line.split(/:(.*)/s)[1].trim()) };
      documents.push(current);
      index += 1;
      continue;
    }

    if (!current || !line.startsWith("    ") || !trimmed.includes(":")) {
      index += 1;
      continue;
    }

    const colon = trimmed.indexOf(":");
    const key = trimmed.slice(0, colon);
    const value = trimmed.slice(colon + 1).trim();
    if (key === "code_paths" || key === "tags") {
      const [items, nextIndex] = readIndexList(lines, index + 1);
      current[key] = items;
      index = nextIndex;
      continue;
    }
    if (key === "summary" && [">", "|"].includes(value)) {
      const [summary, nextIndex] = readIndexSummary(lines, index + 1);
      current[key] = summary;
      index = nextIndex;
      continue;
    }
    current[key] = unquote(value);
    index += 1;
  }

  return documents.map((document) => normalizeIndexDocument(document));
}

function readIndexList(lines, start) {
  const items = [];
  let index = start;
  while (index < lines.length) {
    const line = lines[index];
    const trimmed = line.trim();
    if (!line.startsWith("      - ")) {
      break;
    }
    items.push(unquote(trimmed.slice(2).trim()));
    index += 1;
  }
  return [items, index];
}

function readIndexSummary(lines, start) {
  const parts = [];
  let index = start;
  while (index < lines.length) {
    const line = lines[index];
    if (!line.startsWith("      ")) {
      break;
    }
    const trimmed = line.trim();
    if (trimmed) {
      parts.push(trimmed);
    }
    index += 1;
  }
  return [parts.join(" ").trim(), index];
}

function normalizeIndexDocument(document) {
  const normalized = {};
  for (const field of INDEX_FIELDS) {
    normalized[field] =
      field === "code_paths" || field === "tags"
        ? asList(document[field] ?? [])
        : asText(document[field] ?? "");
  }
  return normalized;
}

function mergeDocuments(existing, updates) {
  const merged = new Map();
  for (const document of existing) {
    merged.set(asText(document.doc_id), { ...document });
  }
  for (const update of updates) {
    merged.set(asText(update.doc_id), { ...update });
  }
  return Array.from(merged.keys())
    .sort()
    .map((docId) => merged.get(docId));
}

function writeIndex(path, documents) {
  mkdirSync(dirname(path), { recursive: true });
  const lines = ["version: 1", "documents:"];

  for (const document of documents) {
    lines.push(`  - doc_id: ${formatScalar(document.doc_id)}`);
    lines.push(`    path: ${formatScalar(document.path)}`);
    lines.push(`    title: ${formatScalar(document.title)}`);
    lines.push(`    status: ${formatScalar(document.status)}`);
    lines.push("    code_paths:");
    for (const codePath of asList(document.code_paths)) {
      lines.push(`      - ${formatScalar(codePath)}`);
    }
    lines.push("    tags:");
    for (const tag of asList(document.tags)) {
      lines.push(`      - ${formatScalar(tag)}`);
    }
    lines.push("    summary: >");
    for (const summaryLine of wrapSummary(asText(document.summary))) {
      lines.push(`      ${summaryLine}`);
    }
  }

  writeFileSync(path, `${lines.join("\n")}\n`, "utf8");
}

function wrapSummary(summary) {
  const normalized = summary.split(/\s+/).filter(Boolean).join(" ");
  if (!normalized) {
    return [""];
  }

  const width = 78;
  const words = normalized.split(" ");
  const lines = [];
  let current = "";
  for (const word of words) {
    const candidate = current ? `${current} ${word}` : word;
    if (candidate.length > width && current) {
      lines.push(current);
      current = word;
    } else {
      current = candidate;
    }
  }
  if (current) {
    lines.push(current);
  }
  return lines;
}

function asText(value) {
  if (Array.isArray(value)) {
    return value.map((part) => String(part)).join(" ").trim();
  }
  return String(value ?? "").trim();
}

function asList(value) {
  if (Array.isArray(value)) {
    return value.map((item) => String(item).trim()).filter(Boolean);
  }
  const text = String(value ?? "").trim();
  return text ? [text] : [];
}

function unquote(value) {
  if (value.length >= 2 && value[0] === value.at(-1) && ["'", '"'].includes(value[0])) {
    return value.slice(1, -1);
  }
  return value;
}

function formatScalar(value) {
  const text = asText(value);
  if (text === "") {
    return '""';
  }

  const needsQuotes =
    text.trim() !== text ||
    "-?:,[]{}#&*!|>'\"%@`".includes(text[0]) ||
    text.includes(": ") ||
    text.includes(" #");
  if (!needsQuotes) {
    return text;
  }

  const escaped = text.replace(/\\/g, "\\\\").replace(/"/g, '\\"');
  return `"${escaped}"`;
}

function resolveUnderRoot(repoRoot, value) {
  return isAbsolute(value) ? resolve(value) : resolve(repoRoot, value);
}

function relativePath(path, repoRoot) {
  const rel = relative(resolve(repoRoot), resolve(path));
  if (!rel.startsWith("..") && !isAbsolute(rel)) {
    return rel.split(sep).join("/");
  }
  return resolve(path).split(sep).join("/");
}

process.exitCode = main();
