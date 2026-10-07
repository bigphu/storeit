// Hộp xác nhận cho việc không hoàn tác được (đăng xuất mọi nơi, gửi email, bỏ thay đổi).
// Vẽ bằng template riêng của ConfirmDialog trong App.vue: bong bóng biểu tượng, tiêu đề màu
import type { ConfirmationOptions } from 'primevue/confirmationoptions'
import type { IconName } from '@/components/icons'

export interface ConfirmOptions {
  title: string
  body: string
  // các dòng hậu quả, trên nền xám ("3 devices are signed in")
  impact?: string[]
  // nhãn nút hành động: động từ + đối tượng
  action: string
  // đỏ: bỏ thay đổi; xanh dương: không hoàn tác được nhưng không phá gì
  danger: boolean
  icon: IconName
}

type Require = (o: ConfirmationOptions) => void
let require: Require | null = null

// App.vue gắn useConfirm().require lúc khởi động
export function bindConfirm(r: Require) {
  require = r
}

export function confirmAction(o: ConfirmOptions): Promise<boolean> {
  return new Promise((resolve) => {
    if (!require) throw new Error('confirmAction: ConfirmDialog is not mounted')
    let settled = false
    const done = (v: boolean) => {
      if (settled) return
      settled = true
      resolve(v)
    }
    require({
      header: o.title,
      message: o.body,
      // template trong App.vue đọc view
      view: o,
      accept: () => done(true),
      reject: () => done(false),
      onHide: () => done(false),
    } as ConfirmationOptions)
  })
}

export function confirmDiscard(): Promise<boolean> {
  return confirmAction({
    title: 'Discard changes?',
    body: 'What you typed in this form is lost.',
    action: 'Discard',
    danger: true,
    icon: 'alert',
  })
}

// mayClose: form không đổi gì thì đóng luôn; còn thay đổi thì hỏi
export async function mayClose(dirty: boolean, ask: () => Promise<boolean> = confirmDiscard): Promise<boolean> {
  return !dirty || ask()
}
