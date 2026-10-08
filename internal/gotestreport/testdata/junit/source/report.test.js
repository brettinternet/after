import { describe, expect, it } from "vitest";

it("pass", () => {
  console.log("synthetic stdout");
  console.error("synthetic stderr");
  expect(1).toBe(1);
});
it("fail", () => expect(1).toBe(2));
it.skip("skip", () => {});
describe("nested", () => {
  it("pass", () => expect(true).toBe(true));
  it("error", () => {
    throw new Error("synthetic runtime error");
  });
});
