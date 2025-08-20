import { decodePayloads } from "./Components";

export function formatAttributesForDisplay(attributes: { [key: string]: any }): string {
  const decoded = decodePayloads(attributes);
  
  // Custom replacer function to handle already-pretty-printed JSON strings in inputs
  const replacer = (key: string, value: any) => {
    if (key === "inputs" && Array.isArray(value)) {
      // For inputs, we want to parse the pretty-printed JSON strings back to objects
      // so they display nicely in the final JSON
      return value.map(input => {
        try {
          return JSON.parse(input);
        } catch {
          return input;
        }
      });
    }
    return value;
  };
  
  return JSON.stringify(decoded, replacer, 2);
}