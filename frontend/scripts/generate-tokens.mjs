import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const scriptDirectory = path.dirname(fileURLToPath(import.meta.url));
const projectRoot = path.resolve(scriptDirectory, "../..");

const inputFile = path.join(projectRoot, "docs/design/tokens.json");
const outputFile = path.join(
  projectRoot,
  "frontend/src/styles/generated/tokens.css"
);

const tokens = JSON.parse(fs.readFileSync(inputFile, "utf8"));

function toKebabCase(value) {
  return value
    .replace(/([a-z0-9])([A-Z])/g, "$1-$2")
    .replace(/_/g, "-")
    .toLowerCase();
}

function tokenName(parts) {
  return `--${parts.map(toKebabCase).join("-")}`;
}

function formatValue(value) {
  if (Array.isArray(value)) {
    const colors = value.map(formatValue).join(", ");
    return `linear-gradient(135deg, ${colors})`;
  }

  if (typeof value === "string") {
    return value.replace(/\{([^}]+)\}/g, (_, reference) => {
      const referencePath = reference.split(".");
      return `var(${tokenName(referencePath)})`;
    });
  }

  return String(value);
}

const rootVariables = [];
const darkVariables = [];

function walk(node, currentPath = []) {
  if (!node || typeof node !== "object") {
    return;
  }

  // A real token has a $value property.
  if (Object.prototype.hasOwnProperty.call(node, "$value")) {
    const value = formatValue(node.$value);

    // Theme tokens get semantic CSS names.
    if (
      currentPath[0] === "themes" &&
      currentPath[1] === "light"
    ) {
      const semanticPath = currentPath.slice(2);

      rootVariables.push(
        `  ${tokenName(semanticPath)}: ${value};`
      );

      return;
    }

    if (
      currentPath[0] === "themes" &&
      currentPath[1] === "dark"
    ) {
      const semanticPath = currentPath.slice(2);

      darkVariables.push(
        `  ${tokenName(semanticPath)}: ${value};`
      );

      return;
    }

    // Core and other non-theme tokens.
    rootVariables.push(
      `  ${tokenName(currentPath)}: ${value};`
    );

    return;
  }

  // Continue through nested groups.
  for (const [key, child] of Object.entries(node)) {
    if (key.startsWith("$")) {
      continue;
    }

    walk(child, [...currentPath, key]);
  }
}

walk(tokens);

const output = `/*
 * GENERATED FILE.
 * Source: docs/design/tokens.json
 * Do not edit manually.
 */

:root {
${rootVariables.join("\n")}
}

.dark {
${darkVariables.join("\n")}
}
`;

fs.mkdirSync(path.dirname(outputFile), { recursive: true });
fs.writeFileSync(outputFile, output, "utf8");

console.log(`Design tokens generated: ${outputFile}`);
console.log(`Root variables: ${rootVariables.length}`);
console.log(`Dark variables: ${darkVariables.length}`);
