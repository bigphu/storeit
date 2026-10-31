<script setup lang="ts">
import AppLogo from '@/components/AppLogo.vue'
import landing from '@/assets/landing.jpg'
</script>

<template>
  <!-- Trang công khai (đăng nhập, quên mật khẩu, đặt mật khẩu): bên trái thẻ ảnh lớn (kho hàng,
       phủ tối ánh xanh) với logo và câu giới thiệu; bên phải form trên nền hoa văn.
       Màn hẹp: thẻ ảnh thành dải trên cùng chỉ có logo -->
  <div class="public app-patterned">
    <aside class="public-art" :style="{ '--landing': `url(${landing})` }">
      <AppLogo class="public-logo" />
      <div class="pitch">
        <p class="eyebrow-light">Asset management</p>
        <h1>Every asset,<br />accounted for.</h1>
        <p class="lede">What you own, who has it and when it comes back, in one calm place.</p>
      </div>
      <a class="credit" href="https://unsplash.com/photos/warehouse-shelving-with-stacked-boxes-yqu6tJkSQ_k" target="_blank" rel="noopener">
        Photo: Arno Senoner / Unsplash
      </a>
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
  grid-template-columns: minmax(0, 1.05fr) minmax(0, 1fr);
}
/* thẻ ảnh: cách mép 14px, bo 28px; ảnh phủ tối từ trên xuống, ánh xanh brand ở góc dưới */
.public-art {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
  margin: 14px 0 14px 14px;
  padding: 2rem 2.25rem 1.5rem;
  border-radius: 28px;
  overflow: hidden;
  color: #fff;
  background:
    radial-gradient(120% 70% at 0% 100%, rgba(16, 185, 129, 0.38), transparent 60%),
    linear-gradient(180deg, rgba(3, 14, 10, 0.55) 0%, rgba(3, 14, 10, 0.72) 45%, rgba(3, 14, 10, 0.94) 100%),
    var(--landing) center / cover no-repeat,
    #06120d;
  box-shadow: 0 20px 50px rgba(0, 0, 0, 0.25);
}
.public-logo {
  color: #fff;
}
/* câu giới thiệu dồn xuống đáy thẻ, trên dòng ghi công ảnh */
.pitch {
  margin-top: auto;
  display: flex;
  flex-direction: column;
  gap: 0.9rem;
  max-width: 30rem;
}
.eyebrow-light {
  margin: 0;
  font-size: 0.8rem;
  font-weight: 600;
  color: #6ee7b7;
}
.pitch h1 {
  margin: 0;
  font: 700 clamp(2.1rem, 3.6vw, 3.1rem) / 1.05 var(--app-display);
  letter-spacing: -0.035em;
}
.lede {
  margin: 0;
  max-width: 24rem;
  font-size: 1rem;
  line-height: 1.5;
  color: rgba(255, 255, 255, 0.72);
}
.credit {
  align-self: flex-end;
  font-size: 0.7rem;
  color: rgba(255, 255, 255, 0.45);
  text-decoration: none;
}
.credit:hover {
  color: rgba(255, 255, 255, 0.8);
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
    margin: 10px 10px 0;
    padding: 1rem 1.25rem;
    border-radius: 20px;
  }
  .pitch,
  .credit {
    display: none;
  }
  .public-main {
    place-items: start center;
    padding: 2rem 1.25rem;
  }
}
</style>
