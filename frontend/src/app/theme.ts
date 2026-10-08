import { definePreset } from '@primeuix/themes'
import Aura from '@primeuix/themes/aura'

// Giữ màu Aura; trạng thái "đang chọn" có hình dạng quen thuộc, tô màu chính: nền xanh
// nhạt cho "đang ở đây", dấu tick xanh cho "đã chọn", gạch dưới xanh cho "mục nào".
// Chữ giữ màu thường; rê chuột lên menu, lựa chọn là xám trung tính.
// Dấu tick của lựa chọn đã chọn và thanh tab của ứng dụng nằm ở base.css / TabBar.vue.

// Lựa chọn trong danh sách: đã chọn không tô nền (có dấu tick), rê chuột/bàn phím tô xám
const listOption = (hover: string) => ({
  option: {
    focusBackground: hover,
    focusColor: '{text.color}',
    selectedBackground: 'transparent',
    selectedFocusBackground: hover,
    selectedColor: '{text.color}',
    selectedFocusColor: '{text.color}',
  },
})
const navHover = (hover: string) => ({ item: { focusBackground: hover, focusColor: '{text.color}' } })

// Rê chuột lên dòng bảng: sắc màu chính rất nhạt (người dùng chọn riêng cho bảng)
const rowHover = 'var(--app-row-hover)'

// Tag đặc: nền là sắc đậm của vai trò, chữ trắng đậm, bo tròn (spec "Tags"); hai theme như nhau
const solidTags = {
  primary: { background: 'var(--app-brand-strong)', color: 'var(--app-on-color)' },
  success: { background: 'var(--app-brand-strong)', color: 'var(--app-on-color)' },
  info: { background: 'var(--app-info-strong)', color: 'var(--app-on-color)' },
  warn: { background: 'var(--app-warn-strong)', color: 'var(--app-on-color)' },
  danger: { background: 'var(--app-danger-strong)', color: 'var(--app-on-color)' },
  secondary: { background: 'var(--app-neutral-strong)', color: 'var(--app-on-color)' },
}

export const StoreItPreset = definePreset(Aura, {
  semantic: {
    colorScheme: {
      light: { list: listOption('{surface.100}'), navigation: navHover('{surface.100}') },
      dark: { list: listOption('{surface.800}'), navigation: navHover('{surface.800}') },
    },
  },
  components: {
    tag: {
      root: { fontSize: '0.76rem', fontWeight: '700', padding: '0.12rem 0.55rem', borderRadius: '999px' },
      colorScheme: { light: solidTags, dark: solidTags },
    },
    datatable: {
      row: { hoverBackground: rowHover, hoverColor: '{text.color}' },
    },
    // Tab trong trang: chữ mờ, tab đang mở chữ thường và gạch dưới 2px màu chính
    tabs: {
      tab: {
        activeColor: '{text.color}',
        activeBorderColor: '{content.border.color}',
      },
      activeBar: { height: '2px', bottom: '-1px', background: '{primary.color}' },
    },
    // Trang hiện tại: nền xanh nhạt, chữ thường
    paginator: {
      navButton: {
        selectedBackground: 'var(--app-selected)',
        selectedColor: '{text.color}',
      },
    },
  },
})
