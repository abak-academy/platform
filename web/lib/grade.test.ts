import { describe, it, expect } from "vitest";
import { formatGrade, gradeOptionsForJenjang, GRADE_GROUPS } from "./grade";

describe("formatGrade", () => {
  it("labels the fixed numeric codes", () => {
    expect(formatGrade(0)).toBe("TK");
    expect(formatGrade(13)).toBe("S1");
    expect(formatGrade(14)).toBe("S2");
    expect(formatGrade(15)).toBe("S3");
  });

  it("keeps school grades as plain numbers", () => {
    expect(formatGrade(1)).toBe("1");
    expect(formatGrade(6)).toBe("6");
    expect(formatGrade(7)).toBe("7");
    expect(formatGrade(12)).toBe("12");
  });

  it("renders blank for missing grades", () => {
    expect(formatGrade(null)).toBe("");
    expect(formatGrade(undefined)).toBe("");
  });
});

describe("gradeOptionsForJenjang", () => {
  it("maps jenjang to its grade range, case-insensitively", () => {
    expect(gradeOptionsForJenjang("TK")).toEqual([0]);
    expect(gradeOptionsForJenjang("sd")).toEqual([1, 2, 3, 4, 5, 6]);
    expect(gradeOptionsForJenjang(" SMP ")).toEqual([7, 8, 9]);
    expect(gradeOptionsForJenjang("SMA")).toEqual([10, 11, 12]);
    expect(gradeOptionsForJenjang("smk")).toEqual([10, 11, 12]);
    expect(gradeOptionsForJenjang("S1")).toEqual([13, 14, 15]);
    expect(gradeOptionsForJenjang("D3")).toEqual([13, 14, 15]);
  });

  it("returns [] for jenjang without a fixed range", () => {
    expect(gradeOptionsForJenjang("PKBM")).toEqual([]);
    expect(gradeOptionsForJenjang("LKP")).toEqual([]);
    expect(gradeOptionsForJenjang("")).toEqual([]);
    expect(gradeOptionsForJenjang(undefined)).toEqual([]);
  });
});

describe("GRADE_GROUPS", () => {
  it("covers TK through S3 exactly once", () => {
    const values = GRADE_GROUPS.flatMap((g) => g.values);
    expect(values).toEqual([0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15]);
  });
});
