/**
 * Bruno YAML validator using the official OpenCollection JSON Schema.
 *
 * Parses YAML via the `yaml` package, determines the item type from
 * `info.type`, and validates against the corresponding schema
 * (HttpRequest, Folder, etc.) from the official OpenCollection v1.0.0
 * JSON Schema.
 *
 * This gives detailed field-level errors instead of a generic
 * "must match one schema" oneOf error.
 */

import { existsSync, readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { createRequire } from 'node:module';

const require = createRequire(import.meta.url);

const Ajv = require('ajv');
const yaml = require('yaml');

const __dirname = dirname(fileURLToPath(import.meta.url));
const SCHEMA_PATH = join(__dirname, 'schema.json');

// --- Load schema and compile per-type validators ---

const schema = JSON.parse(readFileSync(SCHEMA_PATH, 'utf-8'));
const ajv = new Ajv();
ajv.addSchema(schema);

const schemaId = schema.$id ?? '';

// Map info.type → $defs path
const TYPE_VALIDATORS: Record<string, ReturnType<typeof ajv.compile>> = {
  http: ajv.compile({ $ref: `${schemaId}#/$defs/HttpRequest` }),
  folder: ajv.compile({ $ref: `${schemaId}#/$defs/Folder` }),
  graphql: ajv.compile({ $ref: `${schemaId}#/$defs/GraphQLRequest` }),
  grpc: ajv.compile({ $ref: `${schemaId}#/$defs/GrpcRequest` }),
  websocket: ajv.compile({ $ref: `${schemaId}#/$defs/WebSocketRequest` }),
};

// ScriptFile has no info block, handled separately
const scriptValidator = ajv.compile({ $ref: `${schemaId}#/$defs/ScriptFile` });

// --- Public types ---

export interface ValidationError {
  instancePath: string;
  message: string;
}

export interface ValidateOptions {
  /** When true, content is passed directly instead of read from a file */
  isContent?: boolean;
}

/**
 * Validate a Bruno YAML file or content string against the official
 * OpenCollection v1.0.0 schema.
 *
 * The item type is determined from `info.type` (or top-level `type`
 * for ScriptFile). Returns detailed field-level errors.
 */
export function validateBrunoYaml(
  input: string,
  options?: ValidateOptions,
): ValidationError[] {
  let content: string;

  if (options?.isContent) {
    content = input;
  } else {
    if (!existsSync(input)) {
      return [{ instancePath: 'file', message: `File not found: ${input}` }];
    }
    content = readFileSync(input, 'utf-8');
  }

  let doc: Record<string, unknown>;
  try {
    const parsed = yaml.parse(content);
    if (typeof parsed !== 'object' || parsed === null) {
      return [{ instancePath: '', message: 'Expected a YAML mapping (object) at root' }];
    }
    doc = parsed;
  } catch (err) {
    return [{ instancePath: '', message: `YAML parse error: ${String(err)}` }];
  }

  // Determine the item type
  const info = doc.info as Record<string, unknown> | undefined;
  const type = (info?.type ?? doc.type ?? '') as string;

  const validator = type
    ? (TYPE_VALIDATORS[type] ?? null)
    : null;

  if (!validator) {
    return [{ instancePath: '/info/type', message: `Unknown item type: "${type}". Expected one of: http, folder, graphql, grpc, websocket, script` }];
  }

  const valid = validator(doc);
  if (valid) return [];

  return (validator.errors ?? []).map((err: any) => ({
    instancePath: err.instancePath ?? '',
    message: err.message ?? 'Validation failed',
  }));
}
