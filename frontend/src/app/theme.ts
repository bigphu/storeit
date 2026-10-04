import { definePreset } from '@primeuix/themes'
import Aura from '@primeuix/themes/aura'

// Giữ nguyên màu của Aura; chỉ đổi trạng thái "đang chọn" (tab, lựa chọn trong danh sách,
// nút chọn, trang hiện tại) thành khối màu chính đặc với chữ màu tương phản.
// primary.color và primary.contrast.color đã tự đổi theo sáng/tối.
const selectedOption = {
  option: {
    selectedBackground: '{primary.color}',
    selectedFocusBackground: '{primary.hover.color}',
    selectedColor: '{primary.contrast.color}',
    selectedFocusColor: '{primary.contrast.color}',
  },
}

const checkedToggle = {
  root: {
    checkedBackground: '{primary.color}',
    checkedBorderColor: '{primary.color}',
    checkedColor: '{primary.contrast.color}',
  },
  content: { checkedBackground: '{primary.color}' },
  icon: { checkedColor: '{primary.contrast.color}' },
}

export const StoreItPreset = definePreset(Aura, {
  semantic: {
    colorScheme: {
      light: { list: selectedOption },
      dark: { list: selectedOption },
    },
  },
  components: {
    tabs: {
      tab: {
        activeBackground: '{primary.color}',
        activeBorderColor: '{primary.color}',
        activeColor: '{primary.contrast.color}',
      },
    },
    togglebutton: {
      colorScheme: { light: checkedToggle, dark: checkedToggle },
    },
    paginator: {
      navButton: {
        selectedBackground: '{primary.color}',
        selectedColor: '{primary.contrast.color}',
      },
    },
  },
})
