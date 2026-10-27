import { describe, expect, it } from "vitest";
import { parseAffiliatorCsv } from "./affiliator-csv";

describe("parseAffiliatorCsv", () => {
  it("reads rows by header name regardless of column order", () => {
    const { rows, error } = parseAffiliatorCsv("email,name,password\nbudi@example.com,Budi,secret123\n");
    expect(error).toBeUndefined();
    expect(rows).toEqual([{ name: "Budi", email: "budi@example.com", password: "secret123" }]);
  });

  it("keeps commas and escaped quotes inside quoted cells", () => {
    const { rows } = parseAffiliatorCsv('name,email,password\n"Santoso, Budi",b@example.com,"pa""ss,word"\n');
    expect(rows[0]).toEqual({ name: "Santoso, Budi", email: "b@example.com", password: 'pa"ss,word' });
  });

  it("ignores a BOM, CRLF line ends and blank lines", () => {
    const { rows } = parseAffiliatorCsv("﻿name,email,password\r\nA,a@example.com,12345678\r\n\r\n");
    expect(rows).toHaveLength(1);
  });

  it("reports a missing column", () => {
    expect(parseAffiliatorCsv("name,email\nA,a@example.com\n").error).toBe("header");
  });

  it("reports a file with no data rows", () => {
    expect(parseAffiliatorCsv("name,email,password\n").error).toBe("empty");
    expect(parseAffiliatorCsv("").error).toBe("empty");
  });

  it("leaves short rows with empty cells so the server can name the missing field", () => {
    const { rows } = parseAffiliatorCsv("name,email,password\nA\n");
    expect(rows[0]).toEqual({ name: "A", email: "", password: "" });
  });
});
