import { definePreset } from '@primeuix/themes'
import Aura from '@primeuix/themes/aura'

// Giao diện theo kiểu hệ thống của iOS (lấy từ internship-cse): nền xám nhạt, thẻ trắng nổi bằng
// bóng mềm, ô nhập nền xám trong không viền, bo góc lớn, công tắc kiểu iOS. Màu chính vẫn là
// xanh StoreIt (emerald của Aura). Màu nền, đường kẻ, bóng nằm ở base.css (--app-*), preset lấy
// từ đó để hai nơi không lệch nhau.
// Trạng thái "đang chọn": nền xanh nhạt cho "đang ở đây", dấu tick xanh cho "đã chọn", gạch dưới
// xanh cho "mục nào"; rê chuột là xám trung tính. Dấu tick nằm ở base.css, thanh tab ở TabBar.vue.

// Thang xám của iOS (systemGray…): sáng dùng 0–300, tối dùng 700–950
const iosGray = {
  0: '#ffffff',
  50: '#f2f2f7',
  100: '#e5e5ea',
  200: '#d1d1d6',
  300: '#c7c7cc',
  400: '#aeaeb2',
  500: '#8e8e93',
  600: '#636366',
  700: '#48484a',
  800: '#3a3a3c',
  900: '#2c2c2e',
  950: '#1c1c1e',
}

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

// Ô nhập kiểu iOS: nền xám trong, không viền, viền màu chính khi focus
const filledField = {
  background: 'var(--app-fill)',
  disabledBackground: 'var(--app-fill)',
  filledBackground: 'var(--app-fill)',
  filledHoverBackground: 'var(--app-fill)',
  filledFocusBackground: 'var(--app-fill)',
  borderColor: 'transparent',
  hoverBorderColor: 'transparent',
  focusBorderColor: '{primary.color}',
  shadow: 'none',
}

// Rê chuột lên dòng bảng: sắc màu chính rất nhạt (người dùng chọn riêng cho bảng)
const rowHover = 'var(--app-row-hover)'

// Tag nhạt kiểu iOS: nền là sắc soft của vai trò, chữ là sắc ink, bo tròn; hai theme lấy biến
// của theme đó nên đọc rõ ở cả sáng và tối
const softTags = {
  primary: { background: 'var(--app-brand-soft)', color: 'var(--app-brand-ink)' },
  success: { background: 'var(--app-brand-soft)', color: 'var(--app-brand-ink)' },
  info: { background: 'var(--app-info-soft)', color: 'var(--app-info-ink)' },
  warn: { background: 'var(--app-warn-soft)', color: 'var(--app-warn-ink)' },
  danger: { background: 'var(--app-danger-soft)', color: 'var(--app-danger-ink)' },
  secondary: { background: 'var(--app-neutral-soft)', color: 'var(--app-neutral-ink)' },
}

// Màu chính: xanh đậm, chữ trắng ở cả hai theme (Aura tối mặc định là xanh nhạt chữ đen)
const brandPrimary = {
  color: '{primary.600}',
  contrastColor: '#ffffff',
  hoverColor: '{primary.500}',
  activeColor: '{primary.700}',
}

// Điều khiển phân đoạn kiểu iOS (SelectButton, SegmentedFilter): rãnh xám trong, lựa chọn đang
// bật là viên nổi (--app-segment); giống tab chia mục (.section-tabs trong base.css)
const segmentScheme = {
  root: {
    background: 'var(--app-fill)',
    checkedBackground: 'var(--app-fill)',
    hoverBackground: 'var(--app-fill)',
    borderColor: 'transparent',
    checkedBorderColor: 'transparent',
    color: '{text.muted.color}',
    hoverColor: '{text.color}',
    checkedColor: '{text.color}',
  },
  content: { checkedBackground: 'var(--app-segment)' },
  icon: { color: '{text.muted.color}', hoverColor: '{text.color}', checkedColor: '{text.color}' },
}

// Công tắc kiểu iOS: 44×27, núm trắng, bật là xanh
const iosSwitch = {
  root: { background: 'var(--app-bar)', hoverBackground: 'var(--app-bar)', checkedBackground: '{primary.color}', checkedHoverBackground: '{primary.color}' },
  handle: { background: '#ffffff', hoverBackground: '#ffffff', checkedBackground: '#ffffff', checkedHoverBackground: '#ffffff' },
}

export const StoreItPreset = definePreset(Aura, {
  primitive: {
    borderRadius: { none: '0', xs: '4px', sm: '8px', md: '10px', lg: '14px', xl: '18px' },
  },
  semantic: {
    focusRing: { width: '3px', style: 'solid', color: 'color-mix(in srgb, {primary.color} 40%, transparent)', offset: '0' },
    formField: { borderRadius: '10px' },
    overlay: {
      select: { borderRadius: '14px', shadow: 'var(--app-card-shadow)' },
      popover: { borderRadius: '14px', shadow: 'var(--app-card-shadow)' },
      modal: { borderRadius: '18px', shadow: 'var(--app-card-shadow)' },
      navigation: { shadow: 'var(--app-card-shadow)' },
    },
    list: { option: { borderRadius: '8px' } },
    navigation: { item: { borderRadius: '8px' } },
    colorScheme: {
      light: {
        primary: brandPrimary,
        surface: iosGray,
        content: { background: '#ffffff', hoverBackground: 'var(--app-fill)', borderColor: 'var(--app-line)' },
        formField: { ...filledField, placeholderColor: '{surface.500}', iconColor: '{surface.500}' },
        overlay: { select: { borderColor: 'var(--app-line)' }, popover: { borderColor: 'var(--app-line)' }, modal: { borderColor: 'var(--app-line)' } },
        text: { color: '#1c1c1e', mutedColor: 'var(--app-muted)', hoverMutedColor: '#1c1c1e' },
        list: listOption('var(--app-fill)'),
        navigation: navHover('var(--app-fill)'),
      },
      dark: {
        primary: brandPrimary,
        surface: iosGray,
        content: { background: '#1c1c1e', hoverBackground: 'var(--app-fill)', borderColor: 'var(--app-line)' },
        formField: { ...filledField, placeholderColor: '{surface.500}', iconColor: '{surface.500}' },
        overlay: {
          select: { background: '#2c2c2e', borderColor: 'var(--app-line)' },
          popover: { background: '#2c2c2e', borderColor: 'var(--app-line)' },
          modal: { background: '#1c1c1e', borderColor: 'var(--app-line)' },
        },
        text: { color: '#f5f5f7', mutedColor: 'var(--app-muted)', hoverMutedColor: '#f5f5f7' },
        list: listOption('var(--app-fill)'),
        navigation: navHover('var(--app-fill)'),
      },
    },
  },
  components: {
    tag: {
      root: { fontSize: '0.75rem', fontWeight: '600', padding: '0.18rem 0.6rem', borderRadius: '999px' },
      colorScheme: { light: softTags, dark: softTags },
    },
    datatable: {
      row: { hoverBackground: rowHover, hoverColor: '{text.color}' },
      headerCell: { background: 'var(--app-panel-2)' },
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
    // Thẻ (Panel, Card): bo góc lớn, viền mảnh, bóng mềm
    panel: { root: { borderRadius: '16px' } },
    card: { root: { borderRadius: '16px', shadow: 'var(--app-card-shadow)' } },
    dialog: { root: { borderRadius: '20px' } },
    button: { root: { borderRadius: '10px', paddingX: '0.9rem', paddingY: '0.5rem', gap: '0.45rem' } },
    togglebutton: {
      root: { padding: '0', fontWeight: '500' },
      content: { padding: '0.38rem 1rem', borderRadius: '8px', checkedShadow: 'var(--app-segment-shadow)' },
      colorScheme: { light: segmentScheme, dark: segmentScheme },
    },
    selectbutton: { root: { borderRadius: '11px' } },
    toggleswitch: {
      root: { width: '44px', height: '27px', gap: '2px', borderWidth: '0', slideDuration: '0.25s' },
      handle: { size: '23px' },
      colorScheme: { light: iosSwitch, dark: iosSwitch },
    },
  },
})
