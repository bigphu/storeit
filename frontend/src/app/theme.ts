import { definePreset } from '@primeuix/themes'
import Aura from '@primeuix/themes/aura'

// Giữ nguyên màu của Aura; chỉ đổi trạng thái "đang chọn" (tab, lựa chọn trong danh sách,
// trang hiện tại) thành "thẻ nổi": nền sáng hơn nền xung quanh, chữ màu thường.
// Viền mảnh và bóng của thẻ nằm ở base.css (--app-raised-shadow); màu nền thẻ khớp --app-raised.
// SelectButton của Aura vốn đã theo kiểu này nên không đổi.
const raised = { light: '{surface.0}', dark: '{surface.700}' }

const selectedOption = (bg: string) => ({
  option: {
    selectedBackground: bg,
    selectedFocusBackground: bg,
    selectedColor: '{text.color}',
    selectedFocusColor: '{text.color}',
  },
})

export const StoreItPreset = definePreset(Aura, {
  semantic: {
    colorScheme: {
      light: { list: selectedOption(raised.light) },
      dark: { list: selectedOption(raised.dark) },
    },
  },
  components: {
    // Tabs kiểu "segmented": rãnh xám (base.css), tab đang chọn là thẻ nổi; bỏ gạch chân
    tabs: {
      tablist: { borderWidth: '0', background: 'transparent' },
      tab: {
        borderWidth: '0',
        margin: '0',
        padding: '0.4rem 0.9rem',
        background: 'transparent',
        hoverBackground: 'transparent',
        activeColor: '{text.color}',
      },
      activeBar: { height: '0' },
      colorScheme: {
        light: { tab: { activeBackground: raised.light } },
        dark: { tab: { activeBackground: raised.dark } },
      },
    },
    paginator: {
      navButton: { selectedColor: '{text.color}' },
      colorScheme: {
        light: { navButton: { selectedBackground: raised.light } },
        dark: { navButton: { selectedBackground: raised.dark } },
      },
    },
  },
})
