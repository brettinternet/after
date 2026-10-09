import { describe, expect, test } from "bun:test";

test("pass", () => {
  console.log("synthetic stdout");
  console.error("synthetic stderr");
  expect(1).toBe(1);
});
test("fail", () => expect(1).toBe(2));
test.skip("skip", () => {});
describe("nested", () => {
  test("pass", () => expect(true).toBe(true));
  test("error", () => {
    throw new Error("synthetic runtime error");
  });
});
