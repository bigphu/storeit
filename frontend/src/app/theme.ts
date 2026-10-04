import { definePreset } from '@primeuix/themes'
import Aura from '@primeuix/themes/aura'

// Giữ màu Aura; trạng thái "đang chọn" dùng màu trung tính thay cho màu chính:
// xám cho "đang ở đây", dấu tick cho "đã chọn", gạch dưới cho "mục nào". Màu chính chỉ
// dành cho hành động (nút chính, liên kết, focus, checkbox đã tick).
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
const rowHover = 'color-mix(in srgb, {primary.color} 8%, transparent)'

export const StoreItPreset = definePreset(Aura, {
  semantic: {
    colorScheme: {
      light: { list: listOption('{surface.100}'), navigation: navHover('{surface.100}') },
      dark: { list: listOption('{surface.800}'), navigation: navHover('{surface.800}') },
    },
  },
  components: {
    datatable: {
      row: { hoverBackground: rowHover, hoverColor: '{text.color}' },
    },
    // Tab trong trang: chữ mờ, tab đang mở chữ thường và gạch dưới 2px màu chữ
    tabs: {
      tab: {
        activeColor: '{text.color}',
        activeBorderColor: '{content.border.color}',
      },
      activeBar: { height: '2px', bottom: '-1px', background: '{text.color}' },
    },
    // Trang hiện tại: nền xám, chữ thường
    paginator: {
      navButton: { selectedColor: '{text.color}' },
      colorScheme: {
        light: { navButton: { selectedBackground: '{surface.200}' } },
        dark: { navButton: { selectedBackground: '{surface.700}' } },
      },
    },
  },
})
