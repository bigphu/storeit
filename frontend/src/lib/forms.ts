import { ref } from 'vue'
import { ApiError, describeError } from '@/lib/errors'

// useFormErrors: lỗi theo field (errors[].field của problem) và lỗi chung của form
export function useFormErrors() {
  const fields = ref<Record<string, string>>({})
  const general = ref('')

  function clear() {
    fields.value = {}
    general.value = ''
  }

  // set: lỗi có field thì hiện dưới input; còn lại hiện trên form
  function set(err: unknown) {
    clear()
    if (err instanceof ApiError && Object.keys(err.fields).length > 0) {
      fields.value = err.fields
      if (err.detail) general.value = err.detail
      return
    }
    general.value = describeError(err)
  }

  return { fields, general, clear, set }
}
