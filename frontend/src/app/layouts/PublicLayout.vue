<script setup lang="ts">
import AppLogo from '@/components/AppLogo.vue'
</script>

<template>
  <!-- Trang công khai (đăng nhập, quên mật khẩu, đặt mật khẩu): bên trái dải đen chỉ có logo
       (hình minh hoạ thêm sau), bên phải form. Màn hẹp: dải đen thành thanh trên cùng -->
  <div class="public">
    <aside class="public-art">
      <AppLogo class="public-logo" />
    </aside>
    <main class="public-main">
      <div class="public-form">
        <RouterView />
      </div>
    </main>
  </div>
</template>

<style scoped>
.public {
  min-height: 100vh;
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  background: var(--p-content-background);
}
.public-art {
  background: #0a0a0a;
  padding: 2rem 2.25rem;
}
.public-logo {
  color: #ffffff;
}
.public-main {
  display: grid;
  place-items: center;
  padding: 2.5rem 1.5rem;
}
.public-form {
  width: min(100%, 22rem);
}
@media (max-width: 760px) {
  .public {
    grid-template-columns: minmax(0, 1fr);
    grid-template-rows: auto 1fr;
  }
  .public-art {
    padding: 1rem 1.25rem;
  }
  .public-main {
    place-items: start center;
    padding: 2rem 1.25rem;
  }
}
</style>
