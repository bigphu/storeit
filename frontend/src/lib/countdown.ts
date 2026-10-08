// Đếm ngược tạm dừng được (thông báo có Undo: rê chuột thì dừng); thời gian truyền vào
// để thử được không cần đồng hồ thật
export interface Countdown {
  pause(now: number): void
  resume(now: number): void
  // phần còn lại, 0..1
  left(now: number): number
}

export function countdown(life: number, start: number): Countdown {
  let remain = life
  let since: number | null = start
  return {
    pause(now) {
      if (since === null) return
      remain -= now - since
      since = null
    },
    resume(now) {
      if (since === null) since = now
    },
    left(now) {
      const spent = since === null ? 0 : now - since
      return Math.max(0, remain - spent) / life
    },
  }
}

// holds: dừng khi có ít nhất một lý do (chuột ở trên, focus ở trong), chạy lại khi không còn
// lý do nào; rời chuột khi focus vẫn ở trong thì vẫn dừng
export function holds(pause: () => void, resume: () => void) {
  const reasons = new Set<string>()
  return {
    hold(reason: string) {
      if (reasons.size === 0) pause()
      reasons.add(reason)
    },
    release(reason: string) {
      if (!reasons.delete(reason)) return
      if (reasons.size === 0) resume()
    },
  }
}
