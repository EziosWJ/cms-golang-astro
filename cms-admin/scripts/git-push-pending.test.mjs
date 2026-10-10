import assert from "node:assert/strict";
import { test } from "node:test";
import { idlePushSession, nextPushSession, pendingIdentity } from "../src/lib/git-push-pending.ts";

const keys = (...values) => { let index = 0; return () => values[index++]; };

test("首次提交生成新的幂等键", () => {
  assert.deepEqual(pendingIdentity(idlePushSession, undefined, keys("k1")), { key: "k1", id: undefined });
  assert.deepEqual(pendingIdentity(idlePushSession, 7, keys("k1")), { key: "k1", id: 7 });
});

test("同一任务的重试复用原幂等键", () => {
  const session = { pending: { key: "k1", id: 7 }, retryVisible: true };
  assert.deepEqual(pendingIdentity(session, 7, keys("k2")), { key: "k1", id: 7 });
});

test("任务 id 不一致时要求先确认上一请求", () => {
  const session = { pending: { key: "k1", id: 7 }, retryVisible: true };
  assert.equal(pendingIdentity(session, 8, keys("k2")), null);
  assert.equal(pendingIdentity(session, undefined, keys("k2")), null);
});

test("请求结果未知时保留幂等键并要求重试同一请求", () => {
  const session = nextPushSession({ pending: { key: "k1", id: undefined }, retryVisible: false }, "unconfirmed");
  assert.deepEqual(session, { pending: { key: "k1", id: undefined }, retryVisible: true });
});

test("提交成功但详情读取失败时保留原任务与幂等键", () => {
  const pending = pendingIdentity(idlePushSession, undefined, keys("k1"));
  let session = { pending, retryVisible: false };
  session = nextPushSession(session, "accepted");
  session = nextPushSession(session, "detail-failed");
  assert.deepEqual(session.pending, { key: "k1", id: undefined });
  assert.equal(session.retryVisible, true);
  // 重试沿用同一身份：服务端按同一个键重放，返回同一任务而不是新建当前版本任务。
  assert.deepEqual(pendingIdentity(session, undefined, keys("k2")), { key: "k1", id: undefined });
});

test("详情读取成功后清除身份", () => {
  const session = nextPushSession({ pending: { key: "k1", id: 7 }, retryVisible: true }, "detail-loaded");
  assert.deepEqual(session, idlePushSession);
  assert.deepEqual(session.pending, null);
});

test("请求被明确拒绝后允许重新开始", () => {
  const session = nextPushSession({ pending: { key: "k1", id: undefined }, retryVisible: true }, "rejected");
  assert.equal(session.pending, null);
  assert.equal(session.retryVisible, false);
  assert.equal(pendingIdentity(session, undefined, keys("k2")).key, "k2");
});
