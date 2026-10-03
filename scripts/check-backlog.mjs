import assert from "node:assert/strict";
import { existsSync } from "node:fs";

function backlog(...args) {
  const result = Bun.spawnSync(["backlog", ...args, "--json"], { stdout: "pipe", stderr: "pipe" });
  assert.equal(result.exitCode, 0, result.stderr.toString());
  const value = JSON.parse(result.stdout.toString());
  assert.equal(value.schemaVersion, 1, "Unsupported Backlog.md JSON schema");
  return value;
}

const list = backlog("task", "list").tasks;
assert.ok(list.length > 0, "Expected a populated implementation queue");
const tasks = new Map();
for (const row of list) {
  const task = backlog("task", row.id).task;
  assert.ok(!tasks.has(task.id), `Duplicate task ${task.id}`);
  assert.ok(task.description?.trim(), `${task.id}: missing context`);
  assert.ok(task.acceptanceCriteria.length > 0, `${task.id}: no acceptance criteria`);
  assert.ok(task.definitionOfDone.length > 0, `${task.id}: no completion gate`);
  for (const doc of task.documentation) {
    if (!/^[a-z]+:\/\//i.test(doc)) assert.ok(existsSync(doc), `${task.id}: missing ${doc}`);
  }
  if (task.status === "Done") {
    assert.ok(
      task.acceptanceCriteria.every((c) => c.checked),
      `${task.id}: unchecked acceptance`,
    );
    assert.ok(
      task.definitionOfDone.every((c) => c.checked),
      `${task.id}: unchecked DoD`,
    );
  }
  tasks.set(task.id, task);
}
const visited = new Set();
const active = new Set();
function visit(id) {
  assert.ok(tasks.has(id), `Missing dependency ${id}`);
  assert.ok(!active.has(id), `Dependency cycle at ${id}`);
  if (visited.has(id)) return;
  active.add(id);
  for (const dependency of tasks.get(id).dependencies) visit(dependency);
  active.delete(id);
  visited.add(id);
}
for (const id of tasks.keys()) visit(id);
const ready = [...tasks.values()].filter(
  (t) =>
    t.status === "To Do" &&
    t.labels.includes("poc") &&
    t.dependencies.every((id) => tasks.get(id).status === "Done"),
);
console.log(
  `Validated ${tasks.size} tasks; ready POC work: ${ready.map((t) => t.id).join(", ") || "none"}`,
);
