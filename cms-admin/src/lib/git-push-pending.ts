// 手动 Git 推送的幂等身份：一次提交流程始终复用同一个 Idempotency-Key，
// 只有请求被明确拒绝、或任务与详情都已确认时才允许开始新的提交。
export type PushPending = { key: string; id?: number };
export type PushSession = { pending: PushPending | null; retryVisible: boolean };
export type PushOutcome =
  | "rejected" // 服务端明确拒绝（4xx）：请求未被接受
  | "unconfirmed" // 网络失败或 5xx：请求结果未知
  | "accepted" // 服务端已返回任务，详情尚未读到
  | "detail-failed" // 任务已建立，仅详情读取失败
  | "detail-loaded"; // 任务与详情都已确认

export const idlePushSession: PushSession = { pending: null, retryVisible: false };

/**
 * 取本次提交使用的幂等身份。已有未确认请求时必须复用同一 key 与任务 id；
 * 返回 null 表示任务 id 不一致，应先确认上一请求。
 */
export function pendingIdentity(session: PushSession, id: number | undefined, key: () => string): PushPending | null {
  if (session.pending) {
    return session.pending.id === id ? session.pending : null;
  }
  return { key: key(), id };
}

/**
 * 请求结果到会话状态的迁移。只有详情读取成功或请求被明确拒绝才清除身份：
 * “提交成功但详情查询失败”必须保留原任务与幂等键，重试时服务端按同一键重放，
 * 不会新建一个当前版本的任务。
 */
export function nextPushSession(session: PushSession, outcome: PushOutcome): PushSession {
  if (outcome === "detail-loaded" || outcome === "rejected") {
    return idlePushSession;
  }
  return { pending: session.pending, retryVisible: outcome !== "accepted" };
}
